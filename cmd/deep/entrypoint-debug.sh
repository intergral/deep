#!/usr/bin/env bash

#
#     Copyright (C) 2023  Intergral GmbH
#
#     This program is free software: you can redistribute it and/or modify
#     it under the terms of the GNU Affero General Public License as published by
#     the Free Software Foundation, either version 3 of the License, or
#     (at your option) any later version.
#
#     This program is distributed in the hope that it will be useful,
#     but WITHOUT ANY WARRANTY; without even the implied warranty of
#     MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#     GNU Affero General Public License for more details.
#
#     You should have received a copy of the GNU Affero General Public License
#     along with this program.  If not, see <https://www.gnu.org/licenses/>.
#

# entrypoint script used only in the debug image

DLV_OPTIONS="--listen=:2345 --headless=true --api-version=2"
if [ -z "${DEBUG_BLOCK}" ] || [ "${DEBUG_BLOCK}" = 0 ]; then
  echo "start delve in non blocking mode"
  DLV_OPTIONS="${DLV_OPTIONS} --continue"
fi
DLV_LOG_OPTIONS="--log=true --log-output=debugger,gdbwire,lldbout"

exec /dlv ${DLV_OPTIONS} ${DLV_LOG_OPTIONS} exec /deep -- "$@"
