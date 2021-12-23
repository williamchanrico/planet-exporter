// Copyright 2021 - williamchanrico@gmail.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package process

import (
	"context"
	"fmt"

	"github.com/mitchellh/go-ps"
)

// Table maps between Pid and Process name.
type Table map[int]string

// GetProcessTable returns map of current processes Pid to its executable name.
func GetProcessTable(ctx context.Context) (Table, error) {
	processes, err := ps.Processes()
	if err != nil {
		return nil, fmt.Errorf("error retrieving process list: %w", err)
	}

	processTable := make(Table)
	for _, v := range processes {
		processTable[v.Pid()] = v.Executable()
	}

	return processTable, nil
}
