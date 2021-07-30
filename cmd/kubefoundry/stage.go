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
	cobra "github.com/spf13/cobra"
)

var stageCmd = &cobra.Command{
	Use:           "stage",
	Short:         "Build and Push Kubevela application container image",
	Long:          `Build Kubevela Application container simulating CF staging in Docker and if success push the image to the registry `,
	RunE:          stage,
	SilenceUsage:  true,
	SilenceErrors: false,
}

func stage(command *cobra.Command, args []string) error {
	// TODO: add more args (apart of the ones defined in the configuration file)
	err := program.LoadConfig()
	if err == nil {
		err = program.StageAppImage()
	}
	return err
}

func init() {
	Cmd.AddCommand(stageCmd)
}
