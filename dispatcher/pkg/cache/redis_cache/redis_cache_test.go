//     Copyright (C) 2020-2021, IrineSistiana
//
//     This file is part of mosdns.
//
//     mosdns is free software: you can redistribute it and/or modify
//     it under the terms of the GNU General Public License as published by
//     the Free Software Foundation, either version 3 of the License, or
//     (at your option) any later version.
//
//     mosdns is distributed in the hope that it will be useful,
//     but WITHOUT ANY WARRANTY; without even the implied warranty of
//     MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//     GNU General Public License for more details.
//
//     You should have received a copy of the GNU General Public License
//     along with this program.  If not, see <https://www.gnu.org/licenses/>.

package redis_cache

import (
	"github.com/IrineSistiana/mosdns/v2/dispatcher/pkg/pool"
	"github.com/miekg/dns"
	"reflect"
	"testing"
	"time"
)

func Test_RedisValue(t *testing.T) {
	tests := []struct {
		name           string
		storedTime     time.Time
		expirationTime time.Time
		m              *dns.Msg
	}{
		{"test", time.Now(), time.Now().Add(time.Second), new(dns.Msg).SetQuestion("test.", dns.TypeA)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := packRedisData(tt.storedTime, tt.expirationTime, tt.m)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.ReleaseBuf(bytes)

			storedTime, expirationTime, m, err := unpackRedisValue(bytes)
			if err != nil {
				t.Fatal(err)
			}
			if storedTime.Unix() != tt.storedTime.Unix() {
				t.Fatalf("storedTime: want %v, got %v", tt.storedTime, storedTime)
			}
			if expirationTime.Unix() != tt.expirationTime.Unix() {
				t.Fatalf("expirationTime: want %v, got %v", tt.expirationTime, expirationTime)
			}
			if !reflect.DeepEqual(m, tt.m) {
				t.Fatalf("m: want %v, got %v", tt.m, m)
			}
		})
	}
}
