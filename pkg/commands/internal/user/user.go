// Copyright 2017 Joyent, Inc.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package user

import (
	"fmt"
	"github.com/joyent/conch-shell/pkg/util"
	conch "github.com/joyent/go-conch"
	"gopkg.in/jawher/mow.cli.v1"
	"strings"
)

func getSettings(app *cli.Cmd) {
	app.Before = util.BuildAPIAndVerifyLogin
	app.Action = func() {
		settings, err := util.API.GetUserSettings()
		if err != nil {
			util.Bail(err)
		}

		if util.JSON {
			util.JSONOut(settings)
		} else {
			if len(settings) > 0 {
				for k, v := range settings {
					fmt.Printf("%s: %v\n", k, v)
				}
			}
		}
	}
}

func getSetting(app *cli.Cmd) {
	app.Before = util.BuildAPIAndVerifyLogin

	var settingID = app.StringArg("ID", "", "Setting name")
	app.Spec = "ID"

	app.Action = func() {
		setting, err := util.API.GetUserSetting(*settingID)
		if err != nil {
			util.Bail(err)
		}

		if util.JSON {
			util.JSONOut(setting)
		} else {
			fmt.Println(setting)
		}
	}
}

// Login is exported so that it can be used as a first level command as well
// as a nested one
func Login(app *cli.Cmd) {
	var (
		apiURL   = app.StringOpt("api", "https://conch.joyent.us", "The url of the API server")
		user     = app.StringOpt("user u", "", "The user name to log in with")
		password = app.StringOpt("password p", "", "The user's password")
	)

	// BUG(sungo): prompt for data if args are empty
	app.Action = func() {
		api := &conch.Conch{
			BaseURL: strings.TrimRight(*apiURL, "/"),
		}

		if err := api.Login(*user, *password); err != nil {
			util.Bail(err)
		}

		if api.Session == "" {
			util.Bail(conch.ErrNoSessionData)
		}

		util.Config.API = api.BaseURL
		util.Config.Session = api.Session

		if err := util.Config.SerializeToFile(util.Config.Path); err == nil {

			fmt.Printf("Success. Config written to %s\n", util.Config.Path)
		}

	}
}
