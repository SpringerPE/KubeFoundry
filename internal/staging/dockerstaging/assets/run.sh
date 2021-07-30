#!/usr/bin/env bash
[[ -z "$DEBUG" ]] || set -x
# set -euo pipefail

# exec process in bg and keeps waiting if it is running
launch() {
    (
        echo ">>> Launching process with pid=$$: '$@'"
        [[ -z "$DEBUG" ]] || env
        exec $@  2>&1
    ) &
    local pid=$!
    local rc=0
    local i=1
    while [ ${i} -le 60 ]
    do
        if ! kill -0 ${pid} 2> /dev/null
        then
            wait ${pid}
            rc=$?
            break
        fi
        sleep 1
        i=$(( ${i} + 1 ))
    done
    if kill -0 ${pid} 2> /dev/null
    then
        echo ">>> Pid=${pid} running"
        wait
        rc=$?
        echo ">>> Pid=${pid} exited, rc=${rc}"
    elif [ ${rc} -eq 0 ]
    then
        # The process forked, get the children
        children=($(pgrep -P ${pid}))
        (
            echo ">>> Children=${children[0]} of pid=${pid} running"
            while kill -0 ${children[0]}
            do
                sleep 1
            done
        ) &
        wait
        rc=$?
        echo ">>> Children=${children[0]} of pid=${pid} exited"
    else
        echo ">>> Error launching '$@'."
    fi
    return ${rc}
}


# source all files 
load_folder() {
    local dir="${1}"

    local files=()
    if [ -d "${dir}" ]
    then
        # Get list of files by order in the specific path
        while IFS=  read -r -d $'\0' line
        do
            files+=("${line}")
        done < <(find -L ${dir}  -maxdepth 1 -type f -name '*.sh' -print0 | sort -z)

        # launch files
        for filename in "${files[@]}"
        do
            echo ">>> Loading ${filename}"
            source "${filename}"
        done
    fi
}


initd() {
    local dir="${1}"
    local envd="${2}"

    local files=()
    if [ -d "${dir}" ]
    then
        # Get list of files by order in the specific path
        while IFS=  read -r -d $'\0' line
        do
            files+=("${line}")
        done < <(find -L ${dir}  -maxdepth 1 -type f -name '*.sh' -print0 | sort -z)

        # launch files
        for f in "${files[@]}"
        do
            (
                app=$(basename "${f}" | sed -n 's/^\([[:digit:]]\+\)-\(.*\)\.sh$/\2/p')
                if [[ ! -z "${CF_API}" ]]
                then
                    # check if there is a env file with the same name and load it
                    cfenv="${f%.sh}.cfenv"
                    if [[ -f "${cfenv}" ]]
                    then
                        echo ">>> Loading default CF environment variables ..."
                        source "${cfenv}"
                    fi
                fi
                echo ">>> Launching application ${app} ...."
                launch "${f}"
            ) &
            sleep 5
        done
    fi
    wait
}


# run
export HOME="${HOME-/home/vcap}"
export LANG="${LANG-C.UTF-8}"
export USER="${USER-root}"
export TMPDIR="${TMPDIR-/home/vcap/tmp}"
export DEPS_DIR="${DEPS_DIR-/home/vcap/deps}"

load_folder "${HOME}/profile.d"
load_folder "${HOME}/app/.profile.d"
[ -f ${HOME}/app/.profile ] && source .profile
initd ${HOME}/init.d
