// Copyright Joyent, Inc.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package tester

import (
	"fmt"

	"github.com/joyent/conch-shell/pkg/conch"
	"github.com/joyent/conch-shell/pkg/conch/uuid"
	"github.com/joyent/conch-shell/pkg/util"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	ServerPlanName = "Conch v1 Legacy Plan: Server"
	SwitchPlanName = "Conch v1 Legacy Plan: Switch"
)

var (
	UserAgent    string
	API          *conch.Conch
	ServerPlanID uuid.UUID
	SwitchPlanID uuid.UUID
	Validations  map[uuid.UUID]conch.Validation
)

var (
	rootCmd = &cobra.Command{
		Use:     "tester",
		Version: util.Version,
		Short:   "tester is a tool to test the conch api using device reports",
		Long: `
Tester provides the ability to test a Conch API server using device reports. 

The app looks for the config file 'conch_tester.yml' in /etc, /usr/local/etc, and '.'. Options can be provided via the environment (eg 'CONCH_TESTER_DEBUG') or on the command line.

The option list can be a bit intimidating but the app has a few different modes, regardless of the subcommand.

* CLI Logging, in order of precedence: --verbose, --debug, --trace [1]

* JSON Logging, on the CLI: --json [1]

* Pull reports from the database: --db_host, --db_name, --db_user, --db_password
** Optional: --interval, --limit

* Pull reports from a directory of *.json files: --from_directory, --data_directory

* Log to MatterMost: --mattermost, --mattermost_webhook

* The API to test: --conch_api, --conch_user, --conch_password


[1] All logs go to STDERR

`,
	}
)

// Root returns the root command
func Root() *cobra.Command {
	return rootCmd
}

// Execute gets this party started
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	initFlags()
	buildAPI()
	prepEnv()

	UserAgent = fmt.Sprintf("conch %s-%s / API Tester", util.Version, util.GitRev)

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf(
				"Conch %s - API Tester\n"+
					"  Git Revision: %s\n",
				util.Version,
				util.GitRev,
			)
		},
	})
}

func buildAPI() {
	API = &conch.Conch{BaseURL: viper.GetString("conch_api")}
	err := API.Login(
		viper.GetString("conch_user"),
		viper.GetString("conch_password"),
	)

	if err != nil {
		log.Fatalf("error logging into %s : %s", viper.GetString("conch_api"), err)
	}
	API.Debug = viper.GetBool("debug")
	API.Trace = viper.GetBool("trace")

}

func initFlags() {
	flag.String(
		"conch_api",
		"https://staging.conch.joyent.us",
		"URL of the Conch API server to test",
	)

	flag.String(
		"conch_user",
		"",
		"Conch API user name",
	)

	flag.String(
		"conch_password",
		"",
		"Password for Conch API user",
	)

	flag.String(
		"db_host",
		"localhost",
		"Database Hostname",
	)

	flag.String(
		"db_name",
		"conch",
		"Database name",
	)

	flag.String(
		"db_user",
		"conch",
		"Database username",
	)

	flag.String(
		"db_password",
		"conch",
		"Database password",
	)

	flag.Bool(
		"debug",
		false,
		"Debug mode",
	)

	flag.Bool(
		"trace",
		false,
		"Trace mode. This is super loud",
	)

	flag.Bool(
		"verbose",
		false,
		"Verbose logging. Less chatty than debug or trace.",
	)

	flag.String(
		"interval",
		"1 hour",
		"Interval for the database query. Resolves to \"now() - interval '1 hour'\"",
	)

	flag.Bool(
		"json",
		false,
		"Log in json format",
	)

	flag.Int(
		"limit",
		20,
		"Submit a maximum of this many reports",
	)

	flag.String(
		"mattermost_webhook",
		"",
		"Webhook for mattermost",
	)

	flag.Bool(
		"mattermost",
		false,
		"Alert failures to mattermost",
	)

	flag.Bool(
		"from_directory",
		false,
		"Use reports from a directory, rather than the database",
	)

	flag.String(
		"data_directory",
		"",
		"A directory full of device reports",
	)

	viper.SetConfigName("conch_tester")
	viper.AddConfigPath("/etc")
	viper.AddConfigPath("/usr/local/etc")
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("conch_tester")
	viper.AutomaticEnv()

	viper.BindPFlags(flag.CommandLine)
	flag.Parse()

	viper.ReadInConfig()

	if viper.GetBool("trace") {
		log.SetLevel(log.TraceLevel)
	} else if viper.GetBool("debug") {
		log.SetLevel(log.DebugLevel)
	} else if viper.GetBool("verbose") {
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(log.WarnLevel)
	}

	if viper.GetBool("json") {
		log.SetFormatter(&log.JSONFormatter{})
	}

	if viper.GetBool("mattermost") {
		if viper.GetString("mattermost_webhook") == "" {
			log.Fatal("Please provide the mattermost_webhook parameter")
		}
	}

	if viper.GetBool("from_directory") {
		if viper.GetString("data_directory") == "" {
			log.Fatal("Please provide the data_directory parameter")
		}
	}
}

func prepEnv() {
	// Find the IDs for the One True Plans
	plans, err := API.GetValidationPlans()
	if err != nil {
		log.Fatalf("error getting validation plans: %s", err)
	}
	for _, plan := range plans {
		if plan.Name == ServerPlanName {
			ServerPlanID = plan.ID
		} else if plan.Name == SwitchPlanName {
			SwitchPlanID = plan.ID
		}
	}
	if uuid.Equal(SwitchPlanID, uuid.UUID{}) {
		log.Fatalf("failed to find validation plan '%s'", SwitchPlanName)
	}

	if uuid.Equal(ServerPlanID, uuid.UUID{}) {
		log.Fatalf("failed to find validation plan '%s'", ServerPlanName)
	}

	// Build a cache of Validation names and details
	Validations = make(map[uuid.UUID]conch.Validation)
	v, err := API.GetValidations()
	if err != nil {
		log.Fatalf("error getting list of validations: '%s'", err)
	}
	for _, validation := range v {
		Validations[validation.ID] = validation
	}
}
