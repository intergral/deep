/*
 * Copyright (C) 2023  Intergral GmbH
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

package util

import (
	"fmt"
	"net"
	"strings"

	"github.com/go-kit/log/level"

	util_log "github.com/intergral/deep/pkg/util/log"
)

// GetFirstAddressOf returns the first IPv4 address of the supplied interface names, omitting any 169.254.x.x automatic private IPs if possible.
func GetFirstAddressOf(names []string) (string, error) {
	var ipAddr string
	for _, name := range names {
		inf, err := net.InterfaceByName(name)
		if err != nil {
			level.Warn(util_log.Logger).Log("msg", "error getting interface", "inf", name, "err", err)
			continue
		}
		addrs, err := inf.Addrs()
		if err != nil {
			level.Warn(util_log.Logger).Log("msg", "error getting addresses for interface", "inf", name, "err", err)
			continue
		}
		if len(addrs) <= 0 {
			level.Warn(util_log.Logger).Log("msg", "no addresses found for interface", "inf", name, "err", err)
			continue
		}
		if ip := filterIPs(addrs); ip != "" {
			ipAddr = ip
		}
		if strings.HasPrefix(ipAddr, `169.254.`) || ipAddr == "" {
			continue
		}
		return ipAddr, nil
	}
	if ipAddr == "" {
		return "", fmt.Errorf("No address found for %s", names)
	}
	if strings.HasPrefix(ipAddr, `169.254.`) {
		level.Warn(util_log.Logger).Log("msg", "using automatic private ip", "address", ipAddr)
	}
	return ipAddr, nil
}

// filterIPs attempts to return the first non automatic private IP (APIPA / 169.254.x.x) if possible, only returning APIPA if available and no other valid IP is found.
func filterIPs(addrs []net.Addr) string {
	var ipAddr string
	for _, addr := range addrs {
		if v, ok := addr.(*net.IPNet); ok {
			if ip := v.IP.To4(); ip != nil {
				ipAddr = v.IP.String()
				if !strings.HasPrefix(ipAddr, `169.254.`) {
					return ipAddr
				}
			}
		}
	}
	return ipAddr
}
