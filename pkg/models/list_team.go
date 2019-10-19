//  Vikunja is a todo-list application to facilitate your life.
//  Copyright 2018 Vikunja and contributors. All rights reserved.
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with this program.  If not, see <https://www.gnu.org/licenses/>.

package models

import "code.vikunja.io/web"

// TeamList defines the relation between a team and a list
type TeamList struct {
	// The unique, numeric id of this list <-> team relation.
	ID int64 `xorm:"int(11) autoincr not null unique pk" json:"id"`
	// The team id.
	TeamID int64 `xorm:"int(11) not null INDEX" json:"teamID" param:"team"`
	// The list id.
	ListID int64 `xorm:"int(11) not null INDEX" json:"-" param:"list"`
	// The right this team has. 0 = Read only, 1 = Read & Write, 2 = Admin. See the docs for more details.
	Right Right `xorm:"int(11) INDEX not null default 0" json:"right" valid:"length(0|2)" maximum:"2" default:"0"`

	// A unix timestamp when this relation was created. You cannot change this value.
	Created int64 `xorm:"created not null" json:"created"`
	// A unix timestamp when this relation was last updated. You cannot change this value.
	Updated int64 `xorm:"updated not null" json:"updated"`

	web.CRUDable `xorm:"-" json:"-"`
	web.Rights   `xorm:"-" json:"-"`
}

// TableName makes beautiful table names
func (TeamList) TableName() string {
	return "team_list"
}

// TeamWithRight represents a team, combined with rights.
type TeamWithRight struct {
	Team  `xorm:"extends"`
	Right Right `json:"right"`
}

// Create creates a new team <-> list relation
// @Summary Add a team to a list
// @Description Gives a team access to a list.
// @tags sharing
// @Accept json
// @Produce json
// @Security JWTKeyAuth
// @Param id path int true "List ID"
// @Param list body models.TeamList true "The team you want to add to the list."
// @Success 200 {object} models.TeamList "The created team<->list relation."
// @Failure 400 {object} code.vikunja.io/web.HTTPError "Invalid team list object provided."
// @Failure 404 {object} code.vikunja.io/web.HTTPError "The team does not exist."
// @Failure 403 {object} code.vikunja.io/web.HTTPError "The user does not have access to the list"
// @Failure 500 {object} models.Message "Internal error"
// @Router /lists/{id}/teams [put]
func (tl *TeamList) Create(a web.Auth) (err error) {

	// Check if the rights are valid
	if err = tl.Right.isValid(); err != nil {
		return
	}

	// Check if the team exists
	_, err = GetTeamByID(tl.TeamID)
	if err != nil {
		return
	}

	// Check if the list exists
	l := &List{ID: tl.ListID}
	if err := l.GetSimpleByID(); err != nil {
		return err
	}

	// Check if the team is already on the list
	exists, err := x.Where("team_id = ?", tl.TeamID).
		And("list_id = ?", tl.ListID).
		Get(&TeamList{})
	if err != nil {
		return
	}
	if exists {
		return ErrTeamAlreadyHasAccess{tl.TeamID, tl.ListID}
	}

	// Insert the new team
	_, err = x.Insert(tl)
	if err != nil {
		return err
	}

	err = updateListLastUpdated(l)
	return
}

// Delete deletes a team <-> list relation based on the list & team id
// @Summary Delete a team from a list
// @Description Delets a team from a list. The team won't have access to the list anymore.
// @tags sharing
// @Produce json
// @Security JWTKeyAuth
// @Param listID path int true "List ID"
// @Param teamID path int true "Team ID"
// @Success 200 {object} models.Message "The team was successfully deleted."
// @Failure 403 {object} code.vikunja.io/web.HTTPError "The user does not have access to the list"
// @Failure 404 {object} code.vikunja.io/web.HTTPError "Team or list does not exist."
// @Failure 500 {object} models.Message "Internal error"
// @Router /lists/{listID}/teams/{teamID} [delete]
func (tl *TeamList) Delete() (err error) {

	// Check if the team exists
	_, err = GetTeamByID(tl.TeamID)
	if err != nil {
		return
	}

	// Check if the team has access to the list
	has, err := x.Where("team_id = ? AND list_id = ?", tl.TeamID, tl.ListID).
		Get(&TeamList{})
	if err != nil {
		return
	}
	if !has {
		return ErrTeamDoesNotHaveAccessToList{TeamID: tl.TeamID, ListID: tl.ListID}
	}

	// Delete the relation
	_, err = x.Where("team_id = ?", tl.TeamID).
		And("list_id = ?", tl.ListID).
		Delete(TeamList{})
	if err != nil {
		return err
	}

	err = updateListLastUpdated(&List{ID: tl.ListID})
	return
}

// ReadAll implements the method to read all teams of a list
// @Summary Get teams on a list
// @Description Returns a list with all teams which have access on a given list.
// @tags sharing
// @Accept json
// @Produce json
// @Param id path int true "List ID"
// @Param p query int false "The page number. Used for pagination. If not provided, the first page of results is returned."
// @Param s query string false "Search teams by its name."
// @Security JWTKeyAuth
// @Success 200 {array} models.TeamWithRight "The teams with their right."
// @Failure 403 {object} code.vikunja.io/web.HTTPError "No right to see the list."
// @Failure 500 {object} models.Message "Internal error"
// @Router /lists/{id}/teams [get]
func (tl *TeamList) ReadAll(search string, a web.Auth, page int) (interface{}, error) {
	// Check if the user can read the namespace
	l := &List{ID: tl.ListID}
	canRead, err := l.CanRead(a)
	if err != nil {
		return nil, err
	}
	if !canRead {
		return nil, ErrNeedToHaveListReadAccess{ListID: tl.ListID, UserID: a.GetID()}
	}

	// Get the teams
	all := []*TeamWithRight{}
	err = x.
		Table("teams").
		Join("INNER", "team_list", "team_id = teams.id").
		Where("team_list.list_id = ?", tl.ListID).
		Limit(getLimitFromPageIndex(page)).
		Where("teams.name LIKE ?", "%"+search+"%").
		Find(&all)

	return all, err
}

// Update updates a team <-> list relation
// @Summary Update a team <-> list relation
// @Description Update a team <-> list relation. Mostly used to update the right that team has.
// @tags sharing
// @Accept json
// @Produce json
// @Param listID path int true "List ID"
// @Param teamID path int true "Team ID"
// @Param list body models.TeamList true "The team you want to update."
// @Security JWTKeyAuth
// @Success 200 {object} models.TeamList "The updated team <-> list relation."
// @Failure 403 {object} code.vikunja.io/web.HTTPError "The user does not have admin-access to the list"
// @Failure 404 {object} code.vikunja.io/web.HTTPError "Team or list does not exist."
// @Failure 500 {object} models.Message "Internal error"
// @Router /lists/{listID}/teams/{teamID} [post]
func (tl *TeamList) Update() (err error) {

	// Check if the right is valid
	if err := tl.Right.isValid(); err != nil {
		return err
	}

	_, err = x.
		Where("list_id = ? AND team_id = ?", tl.ListID, tl.TeamID).
		Cols("right").
		Update(tl)
	if err != nil {
		return err
	}

	err = updateListLastUpdated(&List{ID: tl.ListID})
	return
}