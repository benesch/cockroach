// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License included
// in the file licenses/BSL.txt and at www.mariadb.com/bsl11.
//
// Change Date: 2022-10-01
//
// On the date above, in accordance with the Business Source License, use
// of this software will be governed by the Apache License, Version 2.0,
// included in the file licenses/APL.txt and at
// https://www.apache.org/licenses/LICENSE-2.0

package opt

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util"
)

func BenchmarkColSet(b *testing.B) {
	// Verify that the wrapper doesn't add overhead (as was the case with earlier
	// go versions which couldn't do mid-stack inlining).
	const n = 50
	b.Run("fastintset", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var c util.FastIntSet
			for j := 0; j < n; j++ {
				c.Add(j)
			}
		}
	})
	b.Run("colset", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var c ColSet
			for j := 0; j < n; j++ {
				c.Add(ColumnID(j))
			}
		}
	})
}
