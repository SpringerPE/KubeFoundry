# Docker CF base container for Kubefoundry
# Build it with: docker build -t app .
# Run like: docker run -ti --rm  -p 8080:8080 app

# https://github.com/buildpacks/packs/blob/main/cf/app.go
# https://github.com/tonistiigi/buildkit-pack

# DOCKER_BUILDKIT=1 docker build --progress=plain  -t java .

ARG BASE=cloudfoundry/cflinuxfs3:latest
FROM "${BASE}" AS staging

ARG APP_NAME
ARG APP_CREATED="now"
ARG APP_VERSION="latest"
ARG APP_HOME="/home/vcap"
ARG APP_PORT=8080
ARG CF_API="https://api.cf"
ARG CF_ORG="undefined"
ARG CF_SPACE="undefined"
ARG CF_VCAP_SERVICES="{}"
ARG CF_MANIFEST=manifest.yml
ARG CF_VARS=vars.yml
ARG CONTEXT_DIR=/app
ARG BUILDPACKS_DIR=/buildpacks


# Env vars needed:
# CF_INSTANCE_ADDR
# CF_INSTANCE_INTERNAL_IP
# CF_INSTANCE_IP
# CF_INSTANCE_PORT
# CF_INSTANCE_PORTS
# CF_STACK
# HOME
# LANG
# MEMORY_LIMIT
# PATH
# PWD
# USER
# VCAP_APPLICATION
# VCAP_SERVICES
# VCAP_PLATFORM_OPTIONS
# CF_SYSTEM_CERT_PATH="/etc/cf-system-certificates"
# CF_INSTANCE_CERT="/etc/cf-instance-credentials/instance.crt"
# CF_INSTANCE_KEY="/etc/cf-instance-credentials/instance.key"


ENV HOME=${APP_HOME} \
    APP_NAME=${APP_NAME} \
    APP_VERSION=${APP_VERSION} \
    APP_PORT=${APP_PORT} \
    CF_API=${CF_API} \
    CF_ORG=${CF_ORG} \
    CF_SPACE=${CF_SPACE} \
    CF_VCAP_SERVICES=${CF_VCAP_SERVICES}

# copy resources
COPY *.py /
WORKDIR ${APP_HOME}

# Add buildpacks
VOLUME ${BUILDPACKS_DIR}
#COPY buildpacks ${BUILDPACKS_DIR}/
# Application
RUN rm -f ${CONTEXT_DIR}
COPY app ${CONTEXT_DIR}

# Run staging
RUN /staging.py \
    --home ${APP_HOME} \
    --contex ${CONTEXT_DIR} \
    --cache ${BUILDPACKS_DIR}/cache \
    --manifest ${CF_MANIFEST} \
    --manifest-vars ${CF_VARS} \
    --link-context \
    ${BUILDPACKS_DIR}

#@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@

ARG BASE=cloudfoundry/cflinuxfs3:latest
FROM "${BASE}"


ARG APP_HOME="/home/vcap"
ARG APP_PORT=8080
ARG CF_API="https://api.cf.local"


# ARG APP_NAME
# ARG APP_CREATED="now"
# ARG APP_VERSION="latest"
# ARG CF_ORG="undefined"
# ARG CF_SPACE="undefined"
# ARG CF_VCAP_SERVICES="{}"

LABEL org.opencontainers.image.ref.name="${APP_NAME}"
LABEL org.opencontainers.image.created="${APP_CREATED}"
LABEL org.opencontainers.image.version="${APP_VERSION}"
LABEL org.opencontainers.image.vendor="Springer Nature EE"
LABEL org.opencontainers.image.description="CloudFoundry staging process in Docker"

LABEL com.springernature.kubefoundry.org="${CF_ORG}"
LABEL com.springernature.kubefoundry.team="${CF_SPACE}"
LABEL com.springernature.kubefoundry.application.name="${APP_NAME}"
LABEL com.springernature.kubefoundry.application.version="${APP_VERSION}"
LABEL com.springernature.kubefoundry.application.date="${APP_CREATED}"

# https://docs.cloudfoundry.org/devguide/deploy-apps/environment-variable.html#app-system-env
# If not defined here, these env vars are defined in the run script
# CF_INSTANCE_IP
# CF_INSTANCE_PORT
# CF_INSTANCE_ADDR
# CF_INSTANCE_GUID=
# CF_INSTANCE_INDEX
# CF_INSTANCE_INTERNAL_IP
# CF_INSTANCE_PORTS
# DATABASE_URL
# HOME
# INSTANCE_INDEX
# INSTANCE_GUID
# LANG
# MEMORY_LIMIT
# PATH
# PORT
# PWD
# TMPDIR
# USER
# VCAP_APPLICATION
# VCAP_SERVICES
# VCAP_PLATFORM_OPTIONS
# CF_SYSTEM_CERT_PATH="/etc/cf-system-certificates"
# CF_INSTANCE_CERT="/etc/cf-instance-credentials/instance.crt"
# CF_INSTANCE_KEY="/etc/cf-instance-credentials/instance.key"

# Undefine CF_API (or set it up to empty string, to disable CF env vars)
# env vars
# ENV APP_NAME=${APP_NAME} \
#     APP_VERSION=${APP_VERSION} \
#     APP_PORT=${APP_PORT} \
#     CF_API=${CF_API} \
#     CF_ORG=${CF_ORG} \
#     CF_SPACE=${CF_SPACE} \
#     CF_VCAP_SERVICES=${CF_VCAP_SERVICES}

ENV HOME=${APP_HOME} \
    LANG=C.UTF-8 \
    USER=root \
    TMPDIR=/home/vcap/tmp \
    DEPS_DIR=/home/vcap/deps \
    PORT=${APP_PORT}

WORKDIR ${APP_HOME}
# copy resources
COPY --from=staging ${APP_HOME} .
COPY --from=staging /run.py /healthcheck.sh /

# Note: Docker healthchecks are not used by K8S!
# This is only useful for platforms based in Docker (or supporting the healthchecks!)
# BUT: In K8S is possible to target a command as a healthcheck, so you only need to
# target /healtcheck.sh and check the return code (in the same way as Docker)
HEALTHCHECK \
    --interval=30s \
    --timeout=10s \
    --start-period=60s \
    --retries=3 \
    CMD /healthcheck.sh

EXPOSE ${APP_PORT}
CMD ["/run.py", "--cf-fake-env", "--manifest-env"]
