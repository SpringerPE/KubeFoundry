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
	"fmt"

	cobra "github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:           "push",
	Short:         "Push application to the PaaS",
	Long:          `Push application with the app based on the manifest parameters`,
	RunE:          push,
	SilenceUsage:  true,
	SilenceErrors: false,
}

func push(command *cobra.Command, args []string) (err error) {
	dst, _ := command.Flags().GetString("destination")
	if dst == "test" {
		err = program.LoadConfig()
		if err == nil {
			err = program.PushApp()
		}
	} else {
		err = fmt.Errorf("Not ready yet!. Not properly IMPLEMENTED. Sorry!")
	}
	return err
}

func init() {
	pushCmd.PersistentFlags().StringP("destination", "d", "", "Where to push the app")
	Cmd.AddCommand(pushCmd)
}
