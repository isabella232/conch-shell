// Copyright 2017 Joyent, Inc.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package util contains common routines used throughout the command base
package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/briandowns/spinner"
	"github.com/joyent/conch-shell/pkg/config"
	conch "github.com/joyent/go-conch"
	"github.com/joyent/go-conch/pgtime"
	"github.com/olekukonko/tablewriter"
	cli "gopkg.in/jawher/mow.cli.v1"
	uuid "gopkg.in/satori/go.uuid.v1"
	"os"
	"regexp"
)

var (
	// UserAgent will be used as the http user agent when making API calls
	UserAgent string

	// JSON tells us if we should output JSON
	JSON bool

	// Config is a global Config object
	Config *config.ConchConfig

	// ActiveProfile represents, well, the active profile
	ActiveProfile *config.ConchProfile

	// API is a global Conch API object
	API *conch.Conch

	// Pretty tells us if we should have pretty output
	Pretty bool

	// Spin is a global Spinner object
	Spin *spinner.Spinner
)

// DateFormat should be used in date formatting calls to ensure uniformity of
// output
const DateFormat = "2006-01-02 15:04:05 -0700 MST"

// TimeStr ensures that all PgTimes are formatted using .Local() and DateFormat
func TimeStr(t pgtime.PgTime) string {
	return t.Local().Format(DateFormat)
}

// MinimalDevice represents a limited subset of Device data, that which we are
// going to present to the user
type MinimalDevice struct {
	ID       string        `json:"id"`
	AssetTag string        `json:"asset_tag"`
	Created  pgtime.PgTime `json:"created,int"`
	LastSeen pgtime.PgTime `json:"last_seen,int"`
	Health   string        `json:"health"`
	Flags    string        `json:"flags"`
	AZ       string        `json:"az"`
	Rack     string        `json:"rack"`
}

// BuildAPIAndVerifyLogin builds a Conch object using the Config data and calls
// VerifyLogin
func BuildAPIAndVerifyLogin() {
	BuildAPI()
	if err := API.VerifyLogin(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// BuildAPI builds a Conch object
func BuildAPI() {
	if ActiveProfile == nil {
		Bail(errors.New("No active profile. Please use 'conch profile' to create or set an active profile"))
	}

	API = &conch.Conch{
		BaseURL: ActiveProfile.BaseURL,
		Session: ActiveProfile.Session,
	}
	if UserAgent != "" {
		API.UA = UserAgent
	}
}

// GetMarkdownTable returns a tablewriter configured to output markdown
// compatible text
func GetMarkdownTable() (table *tablewriter.Table) {
	table = tablewriter.NewWriter(os.Stdout)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	return table
}

// Bail is a --json aware way of dying
func Bail(err error) {
	if JSON {
		j, _ := json.Marshal(struct {
			Error   bool   `json:"error"`
			Message string `json:"message"`
		}{
			true,
			fmt.Sprintf("%v", err),
		})

		fmt.Println(string(j))
	} else {
		fmt.Println(err)
	}
	cli.Exit(1)
}

// DisplayDevices is an abstraction to make sure that the output of
// Devices is uniform, be it tables, json, or full json
func DisplayDevices(devices []conch.Device, fullOutput bool) (err error) {
	minimals := make([]MinimalDevice, 0)
	for _, d := range devices {
		minimals = append(minimals, MinimalDevice{
			d.ID,
			d.AssetTag,
			d.Created,
			d.LastSeen,
			d.Health,
			GenerateDeviceFlags(d),
			d.Location.Datacenter.Name,
			d.Location.Rack.Name,
		})
	}

	if JSON {
		var j []byte
		if fullOutput {
			j, err = json.Marshal(devices)
		} else {
			j, err = json.Marshal(minimals)
		}
		if err != nil {
			return err
		}
		fmt.Println(string(j))
		return nil
	}

	TableizeMinimalDevices(minimals, fullOutput, GetMarkdownTable()).Render()

	return nil
}

// TableizeMinimalDevices is an abstraction to make sure that tables of
// Devices-turned-MinimalDevices are uniform
func TableizeMinimalDevices(devices []MinimalDevice, fullOutput bool, table *tablewriter.Table) *tablewriter.Table {
	if fullOutput {
		table.SetHeader([]string{
			"AZ",
			"Rack",
			"ID",
			"Asset Tag",
			"Created",
			"Last Seen",
			"Health",
			"Flags",
		})
	} else {
		table.SetHeader([]string{
			"ID",
			"Asset Tag",
			"Created",
			"Last Seen",
			"Health",
			"Flags",
		})
	}

	for _, d := range devices {
		lastSeen := ""
		if !d.LastSeen.IsZero() {
			lastSeen = TimeStr(d.LastSeen)
		}

		if fullOutput {
			table.Append([]string{
				d.AZ,
				d.Rack,
				d.ID,
				d.AssetTag,
				TimeStr(d.Created),
				lastSeen,
				d.Health,
				d.Flags,
			})
		} else {
			table.Append([]string{
				d.ID,
				d.AssetTag,
				TimeStr(d.Created),
				lastSeen,
				d.Health,
				d.Flags,
			})
		}
	}

	return table
}

// GenerateDeviceFlags is an abstraction to make sure that the 'flags' field
// for Devices remains uniform
func GenerateDeviceFlags(d conch.Device) (flags string) {
	flags = ""

	if !d.Deactivated.IsZero() {
		flags += "X"
	}

	if !d.Validated.IsZero() {
		flags += "v"
	}

	if !d.Graduated.IsZero() {
		flags += "g"
	}
	return flags
}

// JSONOut marshals an interface to JSON
func JSONOut(thingy interface{}) {
	j, err := json.Marshal(thingy)

	if err != nil {
		Bail(err)
	}

	fmt.Println(string(j))
}

// MagicWorkspaceID takes a string and tries to find a valid UUID. If the
// string is a UUID, it doesn't get checked further. If not, we dig through
// GetWorkspaces() looking for UUIDs that match up to the first hyphen or where
// the workspace name matches the string
func MagicWorkspaceID(wat string) (id uuid.UUID, err error) {
	id, err = uuid.FromString(wat)
	if err == nil {
		return id, err
	}
	// So, it's not a UUID. Let's try for a string name or partial UUID
	workspaces, err := API.GetWorkspaces()
	if err != nil {
		return id, err
	}

	re := regexp.MustCompile(fmt.Sprintf("^%s-", wat))
	for _, w := range workspaces {
		if (w.Name == wat) || re.MatchString(w.ID.String()) {
			return w.ID, nil
		}
	}

	return id, errors.New("Could not find workspace " + wat)
}

// MagicRackID takes a workspace UUID and a string and tries to find a valid
// rack UUID. If the string is a UUID, it doesn't get checked further. If it's
// not a UUID, we dig through GetWorkspaceRacks() looking for UUIDs that match
// up to the first hyphen or where the name matches the string.
func MagicRackID(workspace fmt.Stringer, wat string) (uuid.UUID, error) {
	id, err := uuid.FromString(wat)
	if err == nil {
		return id, err
	}

	// So, it's not a UUID. Let's try for a string name or partial UUID
	racks, err := API.GetWorkspaceRacks(workspace)
	if err != nil {
		return id, err
	}

	re := regexp.MustCompile(fmt.Sprintf("^%s-", wat))
	for _, r := range racks {
		if (r.Name == wat) || re.MatchString(r.ID.String()) {
			return r.ID, nil
		}
	}

	return id, errors.New("Could not find rack " + wat)
}
