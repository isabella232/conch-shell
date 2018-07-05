// Copyright 2017 Joyent, Inc.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conch

import (
	"fmt"
)

// User represents a person able to access the Conch API or UI
type User struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// GetUserSettings returns the results of /user/me/settings
// The return is a map[string]interface{} because the database structure is a
// string name and a jsonb data field.  There is no way for this library to
// know in advanace what's in that data so here there be dragons.
func (c *Conch) GetUserSettings() (map[string]interface{}, error) {
	settings := make(map[string]interface{})

	aerr := &APIError{}
	res, err := c.sling().New().Get("/user/me/settings").Receive(&settings, aerr)
	return settings, c.isHTTPResOk(res, err, aerr)
}

// GetUserSetting returns the results of /user/me/settings/:key
// The return is an interface{} because the database structure is a string name
// and a jsonb data field.  There is no way for this library to know in
// advanace what's in that data so here there be dragons.
func (c *Conch) GetUserSetting(key string) (interface{}, error) {
	var setting interface{}

	aerr := &APIError{}
	res, err := c.sling().New().Get("/user/me/settings/"+key).
		Receive(&setting, aerr)

	return setting, c.isHTTPResOk(res, err, aerr)
}

// SetUserSettings sets the value of *all* user settings via /user/me/settings
func (c *Conch) SetUserSettings(settings map[string]interface{}) error {
	aerr := &APIError{}
	res, err := c.sling().New().
		Post("/user/me/settings").
		BodyJSON(settings).
		Receive(nil, aerr)

	return c.isHTTPResOk(res, err, aerr)
}

// SetUserSetting sets the value of a user setting via /user/me/settings/:name
func (c *Conch) SetUserSetting(name string, value interface{}) error {
	aerr := &APIError{}
	res, err := c.sling().New().
		Post("/user/me/settings/"+name).
		BodyJSON(value).
		Receive(nil, aerr)

	return c.isHTTPResOk(res, err, aerr)
}

// DeleteUserSetting deletes a user setting via /user/me/settings/:name
func (c *Conch) DeleteUserSetting(name string) error {
	aerr := &APIError{}
	res, err := c.sling().New().
		Delete("/user/me/settings/"+name).
		Receive(nil, aerr)

	return c.isHTTPResOk(res, err, aerr)
}

// InviteUser invites a user to a workspace via /workspace/:uuid/user
func (c *Conch) InviteUser(workspaceUUID fmt.Stringer, user string, role string) error {
	body := struct {
		User string `json:"user"`
		Role string `json:"role"`
	}{
		user,
		role,
	}

	aerr := &APIError{}
	res, err := c.sling().New().
		Post("/workspace/"+workspaceUUID.String()+"/user").
		BodyJSON(body).
		Receive(nil, aerr)

	return c.isHTTPResOk(res, err, aerr)
}