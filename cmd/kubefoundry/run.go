// Copyright © 2021 Springer Nature Engineering Enablement, Jose Riguera
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubefoundry

import (
	"strings"

	cobra "github.com/spf13/cobra"
)

var runlocalCmd = &cobra.Command{
	Use:           "run",
	Short:         "Run application locally using docker",
	Long:          `Run one instance of the application on Docker with the same parameters as Kubevela`,
	RunE:          runlocal,
	SilenceUsage:  true,
	SilenceErrors: false,
}

func runlocal(command *cobra.Command, args []string) error {
	envL, _ := command.Flags().GetStringSlice("env")
	env := make(map[string]string)
	for _, e := range envL {
		pair := strings.SplitN(e, "=", 2)
		env[pair[0]] = pair[1]
	}
	// TODO add more args add env arg (and maybe other ones)
	err := program.LoadConfig()
	if err == nil {
		err = program.RunAppImage(env)
	}
	return err
}

func init() {
	runlocalCmd.PersistentFlags().StringSliceP("env", "e", []string{}, "Pass environment variables to the app with format KEY=Value")
	Cmd.AddCommand(runlocalCmd)
}
