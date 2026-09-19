// Copyright 2026 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and limitations
//    under the License.

package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// scaleSet0007 declares only the durable circuit-breaker state added in this
// migration. Keeping the migration model minimal prevents future ScaleSet
// changes from altering this historical upgrade step.
type scaleSet0007 struct {
	MaxCreateAttempts uint
	CreateFailures    uint
}

func (scaleSet0007) TableName() string { return "scale_sets" }

func init() {
	Register(&gormigrate.Migration{
		ID: "0007_scaleset_create_failure_budget",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&scaleSet0007{})
		},
	})
}
