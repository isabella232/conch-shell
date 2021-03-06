// Copyright Joyent, Inc.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package admin

import (
	"fmt"
	"os"
	"sort"
	"text/template"

	gotree "github.com/DiSiqueira/GoTree"
	"github.com/jawher/mow.cli"
	"github.com/joyent/conch-shell/pkg/conch"
	"github.com/joyent/conch-shell/pkg/conch/uuid"
	"github.com/joyent/conch-shell/pkg/util"
)

func listAllUsers(app *cli.Cmd) {
	app.Action = func() {
		users, err := util.API.GetAllUsers()
		if err != nil {
			util.Bail(err)
		}
		if util.JSON {
			util.JSONOut(users)
			return
		}

		sort.Sort(users)

		table := util.GetMarkdownTable()
		table.SetHeader([]string{
			"ID",
			"Name",
			"Email",
			"Created",
			"Last Login",
			"Is Admin",
		})
		for _, u := range users {
			var last string
			if u.LastLogin.IsZero() {
				last = ""
			} else {
				last = util.TimeStr(u.LastLogin)
			}

			isAdmin := ""
			if u.IsAdmin {
				isAdmin = "X"
			}

			table.Append([]string{
				u.ID.String(),
				u.Name,
				u.Email,
				util.TimeStr(u.Created),
				last,
				isAdmin,
			})
		}
		table.Render()
	}
}

func revokeTokens(app *cli.Cmd) {
	var (
		forceOpt   = app.BoolOpt("force", false, "Perform destructive actions")
		revokeAuth = app.BoolOpt("auth-only", false, "Revoke auth tokens, not API tokens. This will force a user to log in again on the website (and old versions of the shell)")
		tokenAuth  = app.BoolOpt("tokens-only", false, "Revoke all API tokens. This will likely break a lot of automation so use this carefully")
		allAuth    = app.BoolOpt("all", false, "The nuclear option. Revoke all auth *and* API tokens, forcing the user to login again *and* to generate new API tokens for automation processes. Use this very carefully")
	)
	app.Spec = "--force (--auth-only | --tokens-only | --all)"

	app.Action = func() {
		if !*forceOpt {
			return
		}

		if *allAuth {
			if err := util.API.RevokeUserTokensAndLogins(UserEmail); err != nil {
				util.Bail(err)
			}

			if !util.JSON {
				fmt.Printf("Login and API tokens revoked for %s.\n", UserEmail)
			}
			return
		}

		if *revokeAuth {
			if err := util.API.RevokeUserLogins(UserEmail); err != nil {
				util.Bail(err)
			}

			if !util.JSON {
				fmt.Printf("Login tokens revoked for %s.\n", UserEmail)
			}
			return
		}
		if *tokenAuth {
			if err := util.API.RevokeUserTokens(UserEmail); err != nil {
				util.Bail(err)
			}

			if !util.JSON {
				fmt.Printf("API tokens revoked for %s.\n", UserEmail)
			}
			return
		}
	}
}

func listTokens(app *cli.Cmd) {
	app.Before = util.BuildAPIAndVerifyLogin

	app.Action = func() {
		tokens, err := util.API.GetUserTokens(UserEmail)
		if err != nil {
			util.Bail(err)
		}
		if util.JSON {
			util.JSONOut(tokens)
			return
		}

		sort.Sort(tokens)

		table := util.GetMarkdownTable()
		table.SetHeader([]string{"Name", "Created", "Last Used"})

		for _, t := range tokens {
			timeStr := ""
			if !t.LastUsed.IsZero() {
				timeStr = util.TimeStr(t.LastUsed)
			}

			table.Append([]string{
				t.Name,
				util.TimeStr(t.Created),
				timeStr,
			})
		}

		table.Render()
	}
}

func getToken(cmd *cli.Cmd) {
	cmd.Before = util.BuildAPIAndVerifyLogin

	var nameArg = cmd.StringArg("NAME", "", "Name for the token")
	cmd.Spec = "NAME"

	cmd.Action = func() {
		token, err := util.API.GetUserToken(UserEmail, *nameArg)
		if err != nil {
			util.Bail(err)
		}

		if util.JSON {
			util.JSONOut(token)
			return
		}

		lastUsed := "[ Never Used ]"
		if !token.LastUsed.IsZero() {
			lastUsed = util.TimeStr(token.LastUsed)
		}

		fmt.Printf(`
Name: %s
Created: %s
Last Used: %s
`,
			token.Name,
			util.TimeStr(token.Created),
			lastUsed,
		)
	}
}

func removeToken(app *cli.Cmd) {
	app.Before = util.BuildAPIAndVerifyLogin

	var nameArg = app.StringArg("NAME", "", "Name for the token")
	app.Spec = "NAME"

	app.Action = func() {
		err := util.API.DeleteUserToken(UserEmail, *nameArg)
		if err != nil {
			util.Bail(err)
		}
	}
}

func deleteUser(app *cli.Cmd) {
	var (
		forceOpt       = app.BoolOpt("force", false, "Perform destructive actions")
		clearTokensOpt = app.BoolOpt("clear-tokens", false, "Purge the user's API tokens")
	)
	app.Spec = "--force [OPTIONS]"

	app.Action = func() {
		if !*forceOpt {
			return
		}

		if err := util.API.DeleteUser(UserEmail, *clearTokensOpt); err != nil {
			util.Bail(err)
		}

		if !util.JSON {
			fmt.Println("User " + UserEmail + " deleted.")
		}
	}
}

func createUser(app *cli.Cmd) {
	var (
		adminOpt = app.BoolOpt("admin", false, "Set user as system admin")
	)
	app.Action = func() {
		if err := util.API.CreateUser(UserEmail, "", "", *adminOpt); err != nil {
			util.Bail(err)
		}

		if !util.JSON {
			if *adminOpt {
				fmt.Println("Admin user " + UserEmail + " created. An email has been sent containing their new password")
			} else {
				fmt.Println("User " + UserEmail + " created. An email has been sent containing their new password")
			}
		}
	}
}

func resetUserPassword(app *cli.Cmd) {
	var tokensOpt = app.BoolOpt("revoke-tokens", false, "Also revoke the user's API tokens")

	app.Action = func() {
		if err := util.API.ResetUserPassword(UserEmail, *tokensOpt); err != nil {
			util.Bail(err)
		}
		if !util.JSON {
			fmt.Println("The password for " + UserEmail + " has been reset. An email has been sent containing their new password")
		}
	}
}

const userTemplate = `
ID: {{ .ID }}
Name: {{.Name}}
Email: {{.Email}}
Is Admin: {{ .IsAdmin }}

Created: {{.Created.Local}}
Last Login: {{.LastLogin.Local}}
{{if len .Workspaces}}
Workspace Permissions Tree:
{{end}}
`

func buildWSTree(
	parents map[string]conch.WorkspacesAndRoles,
	parent uuid.UUID,
	tree *gotree.GTStructure,
) {

	for _, ws := range parents[parent.String()] {
		sub := gotree.GTStructure{}
		sub.Name = fmt.Sprintf("%s / %s (%s)", ws.Name, ws.Role, ws.ID.String())

		buildWSTree(parents, ws.ID, &sub)
		tree.Items = append(tree.Items, sub)
	}
}

func getUser(app *cli.Cmd) {
	app.Action = func() {
		user, err := util.API.GetUserByEmail(UserEmail)
		if err != nil {
			util.Bail(err)
		}
		if util.JSON {
			util.JSONOut(user)
			return
		}

		sort.Sort(user.Workspaces)

		t, err := template.New("up").Parse(userTemplate)
		if err != nil {
			util.Bail(err)
		}

		if err := t.Execute(os.Stdout, user); err != nil {
			util.Bail(err)
		}

		if len(user.Workspaces) > 0 {
			workspaces := make(map[string]conch.WorkspaceAndRole)
			for _, ws := range user.Workspaces {
				workspaces[ws.ID.String()] = ws
			}

			roots := make([]uuid.UUID, 0)

			parents := make(map[string]conch.WorkspacesAndRoles)

			for _, ws := range workspaces {
				if uuid.Equal(ws.RoleVia, uuid.UUID{}) {
					roots = append(roots, ws.ID)
				} else {
					if _, ok := parents[ws.RoleVia.String()]; !ok {
						parents[ws.RoleVia.String()] = make(conch.WorkspacesAndRoles, 0)
					}
					parents[ws.RoleVia.String()] = append(
						parents[ws.RoleVia.String()],
						ws,
					)
					sort.Sort(parents[ws.RoleVia.String()])
				}
			}

			for _, rootID := range roots {
				tree := gotree.GTStructure{}
				root := workspaces[rootID.String()]
				tree.Name = fmt.Sprintf("%s / %s (%s)", root.Name, root.Role, root.ID.String())

				buildWSTree(parents, rootID, &tree)
				gotree.PrintTree(tree)
			}
		}

	}

}

func updateUser(app *cli.Cmd) {
	var (
		emailOpt = app.StringOpt("email", "", "Change the user's email address")
		nameOpt  = app.StringOpt("name", "", "Set the user's name")
	)
	app.Action = func() {

		user, err := util.API.GetUserByEmail(UserEmail)
		if err != nil {
			util.Bail(err)
		}

		// I'm not supporting admin status here because it's not possible to
		// know if the user set the flag to false because they want to revoke
		// admin status or if they just didn't provide it.
		if err := util.API.UpdateUser(
			user.ID,
			*emailOpt,
			*nameOpt,
			user.IsAdmin,
		); err != nil {
			util.Bail(err)
		}

		if !util.JSON {
			fmt.Println("User " + UserEmail + " updated")
		}
	}
}

func promoteUser(app *cli.Cmd) {
	app.Action = func() {

		user, err := util.API.GetUserByEmail(UserEmail)
		if err != nil {
			util.Bail(err)
		}

		if err := util.API.UpdateUser(
			user.ID,
			user.Email,
			user.Name,
			true,
		); err != nil {
			util.Bail(err)
		}

		if !util.JSON {
			fmt.Println("User " + UserEmail + " promoted to system admin")
		}
	}
}

func demoteUser(app *cli.Cmd) {
	app.Action = func() {

		user, err := util.API.GetUserByEmail(UserEmail)
		if err != nil {
			util.Bail(err)
		}

		if err := util.API.UpdateUser(
			user.ID,
			user.Email,
			user.Name,
			false,
		); err != nil {
			util.Bail(err)
		}

		if !util.JSON {
			fmt.Println("User " + UserEmail + " demoted to regular user")
		}
	}
}
