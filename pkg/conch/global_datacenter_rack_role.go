// Copyright Joyent, Inc.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conch

import (
	"fmt"
	uuid "gopkg.in/satori/go.uuid.v1"
	"net/http"
)

// GetGlobalRackRoles fetches a list of all rack roles in the global domain
func (c *Conch) GetGlobalRackRoles() ([]GlobalRackRole, error) {
	r := make([]GlobalRackRole, 0)

	aerr := &APIError{}
	res, err := c.sling().New().Get("/rack_role").Receive(&r, aerr)

	return r, c.isHTTPResOk(res, err, aerr)
}

// GetGlobalRackRole fetches a single rack role in the global domain, by its
// UUID
func (c *Conch) GetGlobalRackRole(id fmt.Stringer) (*GlobalRackRole, error) {
	r := &GlobalRackRole{}

	aerr := &APIError{}
	res, err := c.sling().New().Get("/rack_role/"+id.String()).Receive(&r, aerr)

	return r, c.isHTTPResOk(res, err, aerr)
}

// SaveGlobalRackRole creates or updates a rack role in the global domain,
// based on the presence of an ID
func (c *Conch) SaveGlobalRackRole(r *GlobalRackRole) error {
	if r.Name == "" {
		return ErrBadInput
	}

	if r.RackSize == 0 {
		return ErrBadInput
	}

	var err error
	var res *http.Response
	aerr := &APIError{}

	if uuid.Equal(r.ID, uuid.UUID{}) {
		j := struct {
			Name     string `json:"name"`
			RackSize int    `json:"rack_size"`
		}{
			r.Name,
			r.RackSize,
		}

		res, err = c.sling().New().Post("/rack_role").BodyJSON(j).Receive(&r, aerr)
	} else {
		j := struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			RackSize int    `json:"rack_size"`
		}{
			r.ID.String(),
			r.Name,
			r.RackSize,
		}

		res, err = c.sling().New().Post("/rack_role/"+r.ID.String()).
			BodyJSON(j).Receive(&r, aerr)
	}

	return c.isHTTPResOk(res, err, aerr)
}

// DeleteGlobalRackRole deletes a rack role
func (c *Conch) DeleteGlobalRackRole(id fmt.Stringer) error {
	aerr := &APIError{}
	res, err := c.sling().New().Delete("/rack_role/"+id.String()).Receive(nil, aerr)
	return c.isHTTPResOk(res, err, aerr)
}