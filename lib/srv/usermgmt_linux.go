/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package srv

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/host"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// newHostUsersBackend initializes a new OS specific HostUsersBackend
func newHostUsersBackend() (HostUsersBackend, error) {
	return &HostUsersProvisioningBackend{
		sudoersPath: "/etc/sudoers.d",
	}, nil
}

// Lookup implements host user information lookup
func (*HostUsersProvisioningBackend) Lookup(username string) (*user.User, error) {
	return user.Lookup(username)
}

// UserGIDs returns the list of group IDs for a user
func (*HostUsersProvisioningBackend) UserGIDs(u *user.User) ([]string, error) {
	return u.GroupIds()
}

// LookupGroup host group information lookup
func (*HostUsersProvisioningBackend) LookupGroup(name string) (*user.Group, error) {
	return user.LookupGroup(name)
}

// GetAllUsers returns a full list of users present on a system
func (*HostUsersProvisioningBackend) GetAllUsers() ([]string, error) {
	users, _, err := host.GetAllUsers()
	return users, err
}

// CreateGroup creates a group on a host
func (*HostUsersProvisioningBackend) CreateGroup(name string) error {
	_, err := host.GroupAdd(name)
	return trace.Wrap(err)
}

// CreateUser creates a user on a host
func (*HostUsersProvisioningBackend) CreateUser(name string, groups []string) error {
	_, err := host.UserAdd(name, groups)
	return trace.Wrap(err)
}

// CreateUser creates a user on a host
func (*HostUsersProvisioningBackend) DeleteUser(name string) error {
	code, err := host.UserDel(name)
	if code == host.UserLoggedInExit {
		return trace.Wrap(ErrUserLoggedIn)
	}
	return trace.Wrap(err)
}

// CheckSudoers ensures that a sudoers file to be written is valid
func (*HostUsersProvisioningBackend) CheckSudoers(contents []byte) error {
	err := host.CheckSudoers(contents)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func writeSudoersFile(name string, data []byte) error {
	// from os.Writefile
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0440)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = f.Write(data)
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}
	return trace.Wrap(err)
}

// WriteSudoersFile creates the user's sudoers file.
func (u *HostUsersProvisioningBackend) WriteSudoersFile(username string, contents []byte) error {
	if err := u.CheckSudoers(contents); err != nil {
		return trace.Wrap(err)
	}
	sudoersFilePath := filepath.Join(u.sudoersPath, fmt.Sprintf("%s-%s", "teleport", username))
	if utils.FileExists(sudoersFilePath) {
		return trace.Errorf("sudoers file %q already exists", sudoersFilePath)
	}
	err := writeSudoersFile(sudoersFilePath, contents)
	return trace.Wrap(err)
}

// RemoveSudoersFile deletes a user's sudoers file.
func (u *HostUsersProvisioningBackend) RemoveSudoersFile(username string) error {
	sudoersFilePath := filepath.Join(u.sudoersPath, fmt.Sprintf("%s-%s", "teleport", username))
	if _, err := os.Stat(sudoersFilePath); os.IsNotExist(err) {
		log.Debugf("User %q, did not have sudoers file as it did not exist at path %q",
			username,
			sudoersFilePath)
		return nil
	}
	return trace.Wrap(os.Remove(sudoersFilePath))
}
