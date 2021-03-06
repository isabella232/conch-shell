// Copyright Joyent, Inc.
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
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Bowery/prompt"
	"github.com/blang/semver"
	"github.com/davecgh/go-spew/spew"
	cli "github.com/jawher/mow.cli"
	"github.com/joyent/conch-shell/pkg/conch"
	"github.com/joyent/conch-shell/pkg/config"
	"github.com/olekukonko/tablewriter"
)

var (
	// UserAgent will be used as the http user agent when making API calls
	UserAgent string

	// JSON tells us if we should output JSON
	JSON bool

	IgnoreConfig bool
	Token        string
	BaseURL      string

	// Config is a global Config object
	Config *config.ConchConfig

	ConfigPath string

	// ActiveProfile represents, well, the active profile
	ActiveProfile *config.ConchProfile

	// API is a global Conch API object
	API *conch.Conch

	// Debug decides if we should put the API in debug mode
	// Yes, this is a bit of a kludge
	Debug bool

	// Trace decides if we should trace the HTTP transactions
	// Yes, this is a bit of a kludge
	Trace bool
)

// These variables are provided by the build environment
var (
	Version    string
	GitRev     string
	SemVersion semver.Version

	FlagsDisableApiVersionCheck string // Used in shell development
	FlagsDisableApiTokenCRUD    string // Useful for preventing automations from creating and deleting tokens
	FlagsNoAdmin                string // Useful for preventing automations from accessing admin commands

)

func DisableApiVersionCheck() bool {
	return FlagsDisableApiVersionCheck != "0"
}

func DisableApiTokenCRUD() bool {
	return FlagsDisableApiTokenCRUD != "0"
}

func NoAdmin() bool {
	return FlagsNoAdmin != "0"
}

func init() {
	SemVersion = CleanVersion(Version)
}

// DateFormat should be used in date formatting calls to ensure uniformity of
// output
const DateFormat = "2006-01-02 15:04:05 -0700 MST"

// RefreshTokenTime represent when a JWT token will be refreshed, based on this
// many seconds left on the expiry time
const RefreshTokenTime = 86400

// TimeStr ensures that all Times are formatted using .Local() and DateFormat
func TimeStr(t time.Time) string {
	return t.Local().Format(DateFormat)
}

// BuildAPIAndVerifyLogin builds a Conch object using the Config data and calls
// VerifyLogin
func BuildAPIAndVerifyLogin() {
	BuildAPI()

	if Token != "" {
		ok, err := API.VerifyToken()
		if !ok {
			Bail(err)
		}
		return
	}

	if err := API.VerifyJwtLogin(RefreshTokenTime, false); err != nil {
		Bail(err)
	}

	ActiveProfile.JWT = API.JWT
	WriteConfig()
}

// WriteConfig serializes the Config struct to disk
func WriteConfig() {
	if IgnoreConfig {
		return
	}

	if err := Config.SerializeToFile(Config.Path); err != nil {
		Bail(err)
	}
}

func WriteConfigForce() {
	if err := Config.SerializeToFile(Config.Path); err != nil {
		Bail(err)
	}
}

// BuildAPI builds a Conch object
func BuildAPI() {
	if IgnoreConfig {
		API = &conch.Conch{
			BaseURL: BaseURL,
			Debug:   Debug,
			Trace:   Trace,
			Token:   Token,
		}

	} else {
		if ActiveProfile == nil {
			Bail(errors.New("no active profile. Please use 'conch profile' to create or set an active profile"))
		}

		API = &conch.Conch{
			BaseURL: ActiveProfile.BaseURL,
			JWT:     ActiveProfile.JWT,
			Token:   string(ActiveProfile.Token),
			Debug:   Debug,
			Trace:   Trace,
		}
	}

	if UserAgent != "" {
		API.UA = UserAgent
	}

	version, err := API.GetVersion()
	if err != nil {
		Bail(err)
	}

	if DisableApiVersionCheck() {
		return
	}

	sem := CleanVersion(version)
	minSem := CleanVersion(conch.MinimumAPIVersion)
	maxSem := CleanVersion(conch.BreakingAPIVersion)

	if sem.Major != minSem.Major {
		Bail(fmt.Errorf(
			"cannot continue. the major version of API server '%s' is '%d' and we require '%d'",
			API.BaseURL,
			sem.Major,
			minSem.Major,
		))
	}

	if sem.LT(minSem) || sem.GTE(maxSem) {
		Bail(fmt.Errorf(
			"cannot continue. the API server version '%s' is '%s' and we require >= %s and < %s",
			API.BaseURL,
			sem,
			minSem,
			maxSem,
		))

	}
}

// GetMarkdownTable returns a tablewriter configured to output markdown
// compatible text
func GetMarkdownTable() (table *tablewriter.Table) {
	table = tablewriter.NewWriter(os.Stdout)
	table.SetAutoWrapText(false)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	return table
}

// Bail is a --json aware way of dying
func Bail(err error) {
	var msg string

	switch err {
	case conch.ErrBadInput:
		msg = err.Error() + " -- Internal Error. Please run with --debug and file a Github Issue"

	case conch.ErrNotAuthorized:
		if len(Token) > 0 {
			msg = err.Error() + " -- The API token might be incorrect or revoked"
		} else {
			msg = err.Error() + " -- Running 'profile relogin' might resolve this"
		}

	case conch.ErrMalformedJWT:
		msg = "The server sent a malformed auth token. Please contact the Conch team"

	case conch.ErrLoginFailed:
		msg = "Something unexpected happened during authentication. Please run with --debug and contact the Conch team"

	default:
		msg = err.Error()
	}

	if JSON {
		j, _ := json.Marshal(struct {
			Error   bool   `json:"error"`
			Message string `json:"message"`
		}{
			true,
			msg,
		})

		fmt.Println(string(j))
	} else {
		fmt.Println(msg)
	}

	cli.Exit(1)
}

// DisplayDevices is an abstraction to make sure that the output of
// Devices is uniform, be it tables, json, or full json
func DisplayDevices(devices []conch.Device, fullOutput bool) (err error) {
	if fullOutput {
		filledIn := make([]conch.Device, 0)
		for _, d := range devices {
			if d.Location.Rack.Name == "" {
				// The table renderer only needs the location data so there's no
				// need to go get a full DetailedDevice with its attendant database
				// queries.
				// In my experience, getting the full DetailedDevice doubles this
				// query time [sungo]
				loc, err := API.GetDeviceLocation(d.ID)
				if err != nil {
					return err
				}
				d.Location = loc
			}
			filledIn = append(filledIn, d)
		}
		devices = filledIn
	}

	if JSON {
		if fullOutput {
			JSONOut(devices)
			return nil
		}

		// BUG(sungo) for back compat
		// AZ and Rack were not ported over since they are always zero-value
		// without fullOutput
		output := make([]interface{}, 0)
		for _, d := range devices {
			output = append(output, struct {
				ID        string    `json:"id"`
				AssetTag  string    `json:"asset_tag"`
				Created   time.Time `json:"created"`
				LastSeen  time.Time `json:"last_seen"`
				Health    string    `json:"health"`
				Graduated time.Time `json:"graduated"`
				Validated time.Time `json:"validated"`
				Phase     string    `json:"phase"`
			}{
				d.ID,
				d.AssetTag,
				d.Created,
				d.LastSeen,
				d.Health,
				d.Graduated,
				d.Validated,
				d.Phase,
			})
		}

		JSONOut(output)
		return nil
	}

	table := GetMarkdownTable()

	if fullOutput {
		table.SetHeader([]string{
			"AZ",
			"Rack",
			"ID",
			"Asset Tag",
			"Created",
			"Last Seen",
			"Health",
			"Validated",
			"Graduated",
			"Phase",
		})
	} else {
		table.SetHeader([]string{
			"ID",
			"Asset Tag",
			"Created",
			"Last Seen",
			"Health",
			"Validated",
			"Graduated",
			"Phase",
		})
	}

	for _, d := range devices {
		validated := ""
		if !d.Validated.IsZero() {
			validated = TimeStr(d.Validated.UTC())
		}
		graduated := ""
		if !d.Graduated.IsZero() {
			graduated = TimeStr(d.Graduated.UTC())
		}

		lastSeen := ""
		if !d.LastSeen.IsZero() {
			lastSeen = TimeStr(d.LastSeen.UTC())
		}

		if fullOutput {
			table.Append([]string{
				d.Location.Room.AZ,
				d.Location.Rack.Name,
				d.ID,
				d.AssetTag,
				TimeStr(d.Created.UTC()),
				lastSeen,
				d.Health,
				validated,
				graduated,
				d.Phase,
			})
		} else {
			table.Append([]string{
				d.ID,
				d.AssetTag,
				TimeStr(d.Created.UTC()),
				lastSeen,
				d.Health,
				validated,
				graduated,
				d.Phase,
			})
		}
	}

	table.Render()

	return nil
}

// JSONOut marshals an interface to JSON
func JSONOut(thingy interface{}) {
	j, err := json.Marshal(thingy)

	if err != nil {
		Bail(err)
	}

	fmt.Println(string(j))
}

// JSONOutIndent marshals an interface to indented JSON
func JSONOutIndent(thingy interface{}) {
	j, err := json.MarshalIndent(thingy, "", "     ")

	if err != nil {
		Bail(err)
	}

	fmt.Println(string(j))
}

// IsPasswordSane verifies that the given password follows the current rules
// and restrictions
func IsPasswordSane(password string, profile *config.ConchProfile) error {
	if utf8.RuneCountInString(password) < 12 {
		return errors.New("length must be >= 12")
	}
	if profile != nil {
		if strings.EqualFold(password, profile.User) {
			return errors.New("password cannot match user name")
		}
	}
	return nil
}

// InteractiveForcePasswordChange is an abstraction for the process of
// prompting a user for a new password, validating it, and issuing the API
// calls to execute the change
func InteractiveForcePasswordChange() {
	fmt.Println("You must change your password to continue.")
	fmt.Println("The new password must:")
	fmt.Println("  * Be at least 12 characters long")
	fmt.Println("  * Cannot match the user name or email address on the account")
	fmt.Println()

	var password string
	for {
		s, err := prompt.Password("New Password:")
		if err != nil {
			Bail(err)
		}
		err = IsPasswordSane(s, ActiveProfile)
		if err != nil {
			fmt.Println(err)
		} else {
			password = s
			break
		}

	}
	if err := API.ChangeMyPassword(password, false); err != nil {
		Bail(err)
	}

	if err := API.Login(ActiveProfile.User, password); err != nil {
		Bail(err)
	}

	ActiveProfile.JWT = API.JWT

	WriteConfigForce()
}

// DDP pretty prints a structure to stderr. "Deep Data Printer"
func DDP(v interface{}) {
	spew.Fdump(
		os.Stderr,
		v,
	)
}

func GithubReleaseCheck() {
	gh, err := LatestGithubRelease()
	if (err != nil) && (err != ErrNoGithubRelease) {
		Bail(err)
	}
	if gh.Upgrade {
		os.Stderr.WriteString(fmt.Sprintf(`
A new release is available! You have v%s but %s is available.
The changelog can be viewed via 'conch update changelog'

You can obtain the new release by:
* Running 'conch update self', which will attempt to overwrite the current application
* Manually download the new release at %s

`,
			Version,
			gh.TagName,
			gh.URL,
		))
	}
}

func init() {
	spew.Config = spew.ConfigState{
		Indent:                  "    ",
		SortKeys:                true,
		DisablePointerAddresses: true,
	}
}
