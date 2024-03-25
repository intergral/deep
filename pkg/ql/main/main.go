/*
 * Copyright (C) 2024  Intergral GmbH
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package main

import (
	"fmt"
	"github.com/intergral/deep/pkg/ql"
)

func main() {
	{
		parseString, err := ql.ParseString("trigger{log=\"i am a test log mesg\" file=\"some.py\"}")

		fmt.Printf("%+v\n\n%s\n\n", parseString, err)
	}
	{
		parseString, err := ql.ParseString("log{log=\"i am a test log mesg\" }")

		fmt.Printf("%+v\n\n%s\n\n", parseString, err)
	}
	{
		parseString, err := ql.ParseString("span{}")

		fmt.Printf("%+v\n\n%s\n\n", parseString, err)
	}
	{
		parseString, err := ql.ParseString("metric{log=\"i am a test log mesg\" file=\"some.py\" label=\"a label\"}")

		fmt.Printf("%+v\n\n%s\n\n", parseString, err)
	}
}
