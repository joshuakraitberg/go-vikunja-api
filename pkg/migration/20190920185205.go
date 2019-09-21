// Copyright 2019 Vikunja and contriubtors. All rights reserved.
//
// This file is part of Vikunja.
//
// Vikunja is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Vikunja is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with Vikunja.  If not, see <https://www.gnu.org/licenses/>.

package migration

import (
	"github.com/go-xorm/xorm"
	"src.techknowlogick.com/xormigrate"
)

type task20190920185205 struct {
	PercentDone float64 `xorm:"DOUBLE null" json:"percentDone"`
}

func (task20190920185205) TableName() string {
	return "tasks"
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20190920185205",
		Description: "Add task percent done",
		Migrate: func(tx *xorm.Engine) error {
			return tx.Sync2(task20190920185205{})
		},
		Rollback: func(tx *xorm.Engine) error {
			return dropTableColum(tx, "tasks", "percent_done")
		},
	})
}
