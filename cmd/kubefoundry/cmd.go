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
	"os"

	cli "kubefoundry/internal/program"

	cobra "github.com/spf13/cobra"
)

var (
	program cli.ProgramCLI
	// Version is injected at compile time (from main.go)
	Version string
	// Build is injected at compile time (from main.go)
	Build string
	// Cmd represents the base command when called without any subcommands
	Cmd = &cobra.Command{
		Short:         "Kubefoundry",
		Long:          `Deploy CloudFoundry applications to Kubevela with style`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Hidden:        false,
	}
)

func init() {
	cobra.OnInitialize(initialize)
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	//Cmd.PersistentFlags().StringP("example", "p", ".", "Set example path")
	program = cli.NewProgram(Build, Version, "config", Cmd)
}

// Run adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the Cmd.
func Run(version, build string) {
	Version = version
	Build = build
	if err := Cmd.Execute(); err != nil {
		fmt.Printf("Errors:\n")
		fmt.Printf("\t%s\n\n", err)
		os.Exit(1)
	}
}

// initialize sets up the program
func initialize() {
	program.Init()
}
