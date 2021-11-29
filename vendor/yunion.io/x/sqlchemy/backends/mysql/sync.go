// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mysql

import (
	"fmt"
	"strings"

	"yunion.io/x/log"
	"yunion.io/x/sqlchemy"
)

func (mysql *SMySQLBackend) CommitTableChangeSQL(ts sqlchemy.ITableSpec, changes sqlchemy.STableChanges) []string {
	ret := make([]string, 0)

	for _, idx := range changes.RemvoeIndexes {
		sql := fmt.Sprintf("DROP INDEX `%s` ON `%s`", idx.Name(), ts.Name())
		ret = append(ret, sql)
		log.Infof("%s;", sql)
	}

	alters := make([]string, 0)
	// first check if primary key is modifed
	changePrimary := false
	oldHasPrimary := false
	for _, col := range changes.RemoveColumns {
		if col.IsPrimary() {
			changePrimary = true
			oldHasPrimary = true
		}
	}
	for _, cols := range changes.UpdatedColumns {
		if cols.OldCol.IsPrimary() != cols.NewCol.IsPrimary() {
			changePrimary = true
		}
		if cols.OldCol.IsPrimary() {
			oldHasPrimary = true
		}
	}
	for _, col := range changes.AddColumns {
		if col.IsPrimary() {
			changePrimary = true
		}
	}
	if changePrimary && oldHasPrimary {
		sql := fmt.Sprintf("DROP PRIMARY KEY")
		alters = append(alters, sql)
	}
	/* IGNORE DROP STATEMENT */
	for _, col := range changes.RemoveColumns {
		sql := fmt.Sprintf("DROP COLUMN `%s`", col.Name())
		log.Infof("ALTER TABLE %s %s;", ts.Name(), sql)
		// alters = append(alters, sql)
		// ignore drop statement
		// if the column is auto_increment integer column,
		// then need to drop auto_increment attribute
		if col.IsAutoIncrement() {
			// make sure the column is nullable
			col.SetNullable(true)
			log.Errorf("column %s is auto_increment, drop auto_inrement attribute", col.Name())
			col.SetAutoIncrement(false)
			sql := fmt.Sprintf("MODIFY COLUMN %s", col.DefinitionString())
			alters = append(alters, sql)
		}
		// if the column is not nullable but no default
		// then need to drop the not-nullable attribute
		if !col.IsNullable() && col.Default() == "" {
			col.SetNullable(true)
			sql := fmt.Sprintf("MODIFY COLUMN %s", col.DefinitionString())
			alters = append(alters, sql)
			log.Errorf("column %s is not nullable but no default, drop not nullable attribute", col.Name())
		}
	}
	for _, cols := range changes.UpdatedColumns {
		sql := fmt.Sprintf("MODIFY COLUMN %s", cols.NewCol.DefinitionString())
		alters = append(alters, sql)
	}
	for _, col := range changes.AddColumns {
		sql := fmt.Sprintf("ADD COLUMN %s", col.DefinitionString())
		alters = append(alters, sql)
	}
	if changePrimary {
		primaries := make([]string, 0)
		for _, c := range ts.Columns() {
			if c.IsPrimary() {
				primaries = append(primaries, fmt.Sprintf("`%s`", c.Name()))
			}
		}
		if len(primaries) > 0 {
			sql := fmt.Sprintf("ADD PRIMARY KEY(%s)", strings.Join(primaries, ", "))
			alters = append(alters, sql)
		}
	}

	if len(alters) > 0 {
		sql := fmt.Sprintf("ALTER TABLE `%s` %s;", ts.Name(), strings.Join(alters, ", "))
		ret = append(ret, sql)
	}

	for _, idx := range changes.AddIndexes {
		sql := fmt.Sprintf("CREATE INDEX `%s` ON `%s` (%s)", idx.Name(), ts.Name(), strings.Join(idx.QuotedColumns(), ","))
		ret = append(ret, sql)
		log.Infof("%s;", sql)
	}

	return ret
}