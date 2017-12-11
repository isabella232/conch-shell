// Copyright 2017 Joyent, Inc.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"github.com/mkideal/cli"
)

type getWorkspaceDevicesArgs struct {
	cli.Helper
	Id         string `cli:"*id,uuid" usage:"ID of the workspace (required)"`
	IdsOnly    bool   `cli:"ids-only" usage:"Only retrieve device IDs"`
	Graduated  string `cli:"graduated" usage:"Filter by the 'graduated' field"`
	Health     string `cli:"health" usage:"Filter by the 'health' field using the string provided"`
	FullOutput bool   `cli:"full" usage:"When --json is used and --ids-only is *not* used, provide full data about the devices rather than the normal truncated data"`
}

var GetWorkspaceDevicesCmd = &cli.Command{
	Name: "get_workspace_devices",
	Desc: "Get a list of devices for the given workspace ID",
	Argv: func() interface{} { return new(getWorkspaceDevicesArgs) },
	Fn: func(ctx *cli.Context) error {
		args, _, api, err := GetStarted(&getWorkspaceDevicesArgs{}, ctx)

		if err != nil {
			return err
		}

		argv := args.Local.(*getWorkspaceDevicesArgs)

		devices, err := api.GetWorkspaceDevices(argv.Id, argv.IdsOnly, argv.Graduated, argv.Health)
		if err != nil {
			return err
		}

		return DisplayDevices(devices, args.Global.JSON, argv.FullOutput)
	},
}