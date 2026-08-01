package api_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/natrimmer/kvweb/internal/testenv"
)

func TestListEndpoints(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		const key = "crud:list"

		t.Run("AppendAndPrepend", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/list", map[string]any{"value": "b"}).ExpectOK()
			h.Post(h.KeyPath(key)+"/list", map[string]any{"value": "c", "position": "tail"}).ExpectOK()
			h.Post(h.KeyPath(key)+"/list", map[string]any{"value": "a", "position": "head"}).ExpectOK()

			if items := h.ListItems(key); !slices.Equal(items, []string{"a", "b", "c"}) {
				t.Errorf("list = %v, want [a b c]", items)
			}
		})

		t.Run("UnknownPositionAppends", func(t *testing.T) {
			// Anything other than "head" is treated as a tail push.
			h.Post(h.KeyPath(key)+"/list", map[string]any{"value": "d", "position": "sideways"}).ExpectOK()
			items := h.ListItems(key)
			if items[len(items)-1] != "d" {
				t.Errorf("list = %v, want d appended at the end", items)
			}
		})

		t.Run("SetByIndex", func(t *testing.T) {
			h.Put(h.KeyPath(key)+"/list/1", map[string]any{"value": "B"}).ExpectOK()
			if items := h.ListItems(key); items[1] != "B" {
				t.Errorf("list = %v, want B at index 1", items)
			}
		})

		t.Run("SetOutOfRange", func(t *testing.T) {
			h.Put(h.KeyPath(key)+"/list/999", map[string]any{"value": "x"}).
				ExpectStatus(http.StatusInternalServerError)
		})

		t.Run("RemoveByIndex", func(t *testing.T) {
			h.Delete(h.KeyPath(key) + "/list/1").ExpectOK()
			if items := h.ListItems(key); !slices.Equal(items, []string{"a", "c", "d"}) {
				t.Errorf("list = %v, want [a c d]", items)
			}
		})

		t.Run("RemoveOutOfRange", func(t *testing.T) {
			h.Delete(h.KeyPath(key) + "/list/999").
				ExpectStatus(http.StatusInternalServerError)
		})

		t.Run("RemoveKeepsDuplicateValues", func(t *testing.T) {
			dupes := "crud:list-dupes"
			h.SeedList(dupes, "same", "same", "same")
			h.Delete(h.KeyPath(dupes) + "/list/0").ExpectOK()
			if items := h.ListItems(dupes); !slices.Equal(items, []string{"same", "same"}) {
				t.Errorf("list = %v, want two remaining duplicates", items)
			}
		})

		t.Run("EmptyValueIsAllowed", func(t *testing.T) {
			empty := "crud:list-empty"
			h.Post(h.KeyPath(empty)+"/list", map[string]any{"value": ""}).ExpectOK()
			if items := h.ListItems(empty); !slices.Equal(items, []string{""}) {
				t.Errorf("list = %q, want one empty element", items)
			}
		})
	})
}

func TestSetEndpoints(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		const key = "crud:set"

		t.Run("Add", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/set", map[string]any{"member": "alpha"}).ExpectOK()
			h.Post(h.KeyPath(key)+"/set", map[string]any{"member": "beta"}).ExpectOK()

			members := h.SetMembers(key)
			slices.Sort(members)
			if !slices.Equal(members, []string{"alpha", "beta"}) {
				t.Errorf("members = %v, want [alpha beta]", members)
			}
		})

		t.Run("DuplicateIsConflict", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/set", map[string]any{"member": "alpha"}).
				ExpectError(http.StatusConflict, "already exists")
		})

		t.Run("EmptyMemberIsRejected", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/set", map[string]any{"member": ""}).
				ExpectError(http.StatusBadRequest, "cannot be empty")
		})

		t.Run("Rename", func(t *testing.T) {
			h.Patch(h.KeyPath(key, "set", "alpha"), map[string]any{"newMember": "gamma"}).ExpectOK()
			members := h.SetMembers(key)
			slices.Sort(members)
			if !slices.Equal(members, []string{"beta", "gamma"}) {
				t.Errorf("members = %v, want [beta gamma]", members)
			}
		})

		t.Run("RenameMissingMember", func(t *testing.T) {
			h.Patch(h.KeyPath(key, "set", "ghost"), map[string]any{"newMember": "x"}).
				ExpectError(http.StatusNotFound, "Member does not exist")
		})

		t.Run("RenameOntoExistingMember", func(t *testing.T) {
			h.Patch(h.KeyPath(key, "set", "gamma"), map[string]any{"newMember": "beta"}).
				ExpectError(http.StatusConflict, "already exists")
		})

		t.Run("RenameRequiresANewMember", func(t *testing.T) {
			h.Patch(h.KeyPath(key, "set", "gamma"), map[string]any{"newMember": ""}).
				ExpectError(http.StatusBadRequest, "cannot be empty")
		})

		t.Run("Remove", func(t *testing.T) {
			h.Delete(h.KeyPath(key, "set", "gamma")).ExpectOK()
			if members := h.SetMembers(key); !slices.Equal(members, []string{"beta"}) {
				t.Errorf("members = %v, want [beta]", members)
			}
		})

		t.Run("MembersNeedingEscaping", func(t *testing.T) {
			escaped := "crud:set-escaped"
			member := "member/with spaces?and#symbols"
			h.Post(h.KeyPath(escaped)+"/set", map[string]any{"member": member}).ExpectOK()

			h.Patch(h.KeyPath(escaped, "set", member), map[string]any{"newMember": "renamed"}).ExpectOK()
			if members := h.SetMembers(escaped); !slices.Equal(members, []string{"renamed"}) {
				t.Errorf("members = %v, want [renamed]", members)
			}

			h.Delete(h.KeyPath(escaped, "set", "renamed")).ExpectOK()
			if members := h.SetMembers(escaped); len(members) != 0 {
				t.Errorf("members = %v, want none", members)
			}
		})
	})
}

func TestHashEndpoints(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		const key = "crud:hash"

		t.Run("SetField", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/hash", map[string]any{"field": "name", "value": "Alice"}).ExpectOK()
			h.Post(h.KeyPath(key)+"/hash", map[string]any{"field": "age", "value": "30"}).ExpectOK()

			fields := h.HashFields(key)
			if fields["name"] != "Alice" || fields["age"] != "30" {
				t.Errorf("fields = %v, want name=Alice age=30", fields)
			}
		})

		t.Run("SetFieldOverwrites", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/hash", map[string]any{"field": "name", "value": "Bob"}).ExpectOK()
			if got := h.HashFields(key)["name"]; got != "Bob" {
				t.Errorf("name = %q, want Bob", got)
			}
		})

		t.Run("EmptyFieldNameIsRejected", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/hash", map[string]any{"field": "", "value": "x"}).
				ExpectError(http.StatusBadRequest, "cannot be empty")
		})

		t.Run("EmptyValueIsAllowed", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/hash", map[string]any{"field": "blank", "value": ""}).ExpectOK()
			if _, ok := h.HashFields(key)["blank"]; !ok {
				t.Error("the empty-valued field was not stored")
			}
		})

		t.Run("Rename", func(t *testing.T) {
			var body struct {
				Status string `json:"status"`
				Value  string `json:"value"`
			}
			h.Patch(h.KeyPath(key, "hash", "name"), map[string]any{"newField": "full_name"}).
				ExpectOK().Decode(&body)
			// The response carries the value across so the editor need not refetch.
			if body.Value != "Bob" {
				t.Errorf("value = %q, want Bob", body.Value)
			}
			fields := h.HashFields(key)
			if _, ok := fields["name"]; ok {
				t.Error("the old field still exists")
			}
			if fields["full_name"] != "Bob" {
				t.Errorf("full_name = %q, want Bob", fields["full_name"])
			}
		})

		t.Run("RenameMissingField", func(t *testing.T) {
			h.Patch(h.KeyPath(key, "hash", "ghost"), map[string]any{"newField": "x"}).
				ExpectError(http.StatusNotFound, "Field does not exist")
		})

		t.Run("RenameOntoExistingField", func(t *testing.T) {
			h.Patch(h.KeyPath(key, "hash", "full_name"), map[string]any{"newField": "age"}).
				ExpectError(http.StatusConflict, "already exists")
		})

		t.Run("RenameRequiresANewName", func(t *testing.T) {
			h.Patch(h.KeyPath(key, "hash", "full_name"), map[string]any{"newField": ""}).
				ExpectError(http.StatusBadRequest, "cannot be empty")
		})

		t.Run("Remove", func(t *testing.T) {
			h.Delete(h.KeyPath(key, "hash", "age")).ExpectOK()
			if _, ok := h.HashFields(key)["age"]; ok {
				t.Error("the field is still present after delete")
			}
		})

		t.Run("FieldsNeedingEscaping", func(t *testing.T) {
			escaped := "crud:hash-escaped"
			field := "field/with spaces?and#symbols"
			h.Post(h.KeyPath(escaped)+"/hash", map[string]any{"field": field, "value": "v"}).ExpectOK()
			if got := h.HashFields(escaped)[field]; got != "v" {
				t.Fatalf("field %q = %q, want v", field, got)
			}
			h.Patch(h.KeyPath(escaped, "hash", field), map[string]any{"newField": "renamed"}).ExpectOK()
			h.Delete(h.KeyPath(escaped, "hash", "renamed")).ExpectOK()
			if fields := h.HashFields(escaped); len(fields) != 0 {
				t.Errorf("fields = %v, want none", fields)
			}
		})
	})
}

func TestZSetEndpoints(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		const key = "crud:zset"

		t.Run("Add", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/zset", map[string]any{"member": "alice", "score": 10.5}).ExpectOK()
			h.Post(h.KeyPath(key)+"/zset", map[string]any{"member": "bob", "score": 20}).ExpectOK()

			members := h.ZSetMembers(key)
			if len(members) != 2 || members[0].Member != "alice" || members[0].Score != 10.5 {
				t.Errorf("members = %+v, want alice at 10.5 first", members)
			}
		})

		t.Run("AddWithoutScoreDefaultsToZero", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/zset", map[string]any{"member": "zero"}).ExpectOK()
			for _, m := range h.ZSetMembers(key) {
				if m.Member == "zero" && m.Score != 0 {
					t.Errorf("zero score = %v, want 0", m.Score)
				}
			}
		})

		t.Run("AddOverwritesScore", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/zset", map[string]any{"member": "alice", "score": 99}).ExpectOK()
			for _, m := range h.ZSetMembers(key) {
				if m.Member == "alice" && m.Score != 99 {
					t.Errorf("alice score = %v, want 99", m.Score)
				}
			}
		})

		t.Run("EmptyMemberIsRejected", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/zset", map[string]any{"member": "", "score": 1}).
				ExpectError(http.StatusBadRequest, "cannot be empty")
		})

		t.Run("IncrementScore", func(t *testing.T) {
			var body struct {
				Score float64 `json:"score"`
			}
			h.Post(h.KeyPath(key, "zset", "bob", "incr"), map[string]any{"amount": 5}).
				ExpectOK().Decode(&body)
			if body.Score != 25 {
				t.Errorf("score = %v, want 25", body.Score)
			}

			h.Post(h.KeyPath(key, "zset", "bob", "incr"), map[string]any{"amount": -2.5}).
				ExpectOK().Decode(&body)
			if body.Score != 22.5 {
				t.Errorf("score = %v, want 22.5", body.Score)
			}
		})

		t.Run("IncrementCreatesMissingMember", func(t *testing.T) {
			var body struct {
				Score float64 `json:"score"`
			}
			h.Post(h.KeyPath(key, "zset", "newcomer", "incr"), map[string]any{"amount": 7}).
				ExpectOK().Decode(&body)
			if body.Score != 7 {
				t.Errorf("score = %v, want 7", body.Score)
			}
		})

		t.Run("Rename", func(t *testing.T) {
			var body struct {
				Score float64 `json:"score"`
			}
			h.Patch(h.KeyPath(key, "zset", "alice"), map[string]any{"newMember": "alicia"}).
				ExpectOK().Decode(&body)
			if body.Score != 99 {
				t.Errorf("returned score = %v, want the original 99", body.Score)
			}
			for _, m := range h.ZSetMembers(key) {
				if m.Member == "alice" {
					t.Error("the old member still exists")
				}
				if m.Member == "alicia" && m.Score != 99 {
					t.Errorf("alicia score = %v, want 99", m.Score)
				}
			}
		})

		t.Run("RenameMissingMember", func(t *testing.T) {
			h.Patch(h.KeyPath(key, "zset", "ghost"), map[string]any{"newMember": "x"}).
				ExpectError(http.StatusNotFound, "Member does not exist")
		})

		t.Run("RenameOntoExistingMember", func(t *testing.T) {
			h.Patch(h.KeyPath(key, "zset", "alicia"), map[string]any{"newMember": "bob"}).
				ExpectError(http.StatusConflict, "already exists")
		})

		t.Run("Remove", func(t *testing.T) {
			h.Delete(h.KeyPath(key, "zset", "alicia")).ExpectOK()
			for _, m := range h.ZSetMembers(key) {
				if m.Member == "alicia" {
					t.Error("the member is still present after delete")
				}
			}
		})
	})
}

func TestGeoEndpoints(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		const key = "crud:geo"

		t.Run("Add", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/geo", map[string]any{
				"member": "palermo", "longitude": 13.361389, "latitude": 38.115556,
			}).ExpectOK()

			var resp struct {
				Value []geoMember `json:"value"`
			}
			h.Get(h.KeyPath(key) + "/geo").ExpectOK().Decode(&resp)
			if len(resp.Value) != 1 || resp.Value[0].Member != "palermo" {
				t.Fatalf("value = %+v, want palermo", resp.Value)
			}
		})

		t.Run("EmptyMemberIsRejected", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/geo", map[string]any{
				"member": "", "longitude": 0, "latitude": 0,
			}).ExpectError(http.StatusBadRequest, "cannot be empty")
		})

		t.Run("CoordinateBounds", func(t *testing.T) {
			cases := []struct {
				name     string
				lon, lat float64
				wantOK   bool
			}{
				{"WestEdge", -180, 0, true},
				{"EastEdge", 180, 0, true},
				{"NorthEdge", 0, 85.05112878, true},
				{"SouthEdge", 0, -85.05112878, true},
				{"TooFarWest", -180.1, 0, false},
				{"TooFarEast", 180.1, 0, false},
				{"TooFarNorth", 0, 85.1, false},
				{"TooFarSouth", 0, -85.1, false},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					resp := h.Post(h.KeyPath(key)+"/geo", map[string]any{
						"member": tc.name, "longitude": tc.lon, "latitude": tc.lat,
					})
					if tc.wantOK {
						resp.ExpectOK()
					} else {
						resp.ExpectStatus(http.StatusBadRequest)
					}
				})
			}
		})

		t.Run("RemoveUsesTheZSetRoute", func(t *testing.T) {
			// Geo members are deleted through the zset endpoint; the geo view
			// has no delete of its own.
			h.Delete(h.KeyPath(key, "zset", "palermo")).ExpectOK()

			var resp struct {
				Value []geoMember `json:"value"`
			}
			h.Get(h.KeyPath(key) + "/geo").ExpectOK().Decode(&resp)
			for _, m := range resp.Value {
				if m.Member == "palermo" {
					t.Error("palermo is still present after delete")
				}
			}
		})
	})
}

func TestStreamEndpoints(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		const key = "crud:stream"

		var firstID string

		t.Run("Add", func(t *testing.T) {
			var body struct {
				Status string `json:"status"`
				ID     string `json:"id"`
			}
			h.Post(h.KeyPath(key)+"/stream", map[string]any{
				"fields": map[string]string{"event": "created", "user": "alice"},
			}).ExpectOK().Decode(&body)

			if body.ID == "" {
				t.Fatal("no entry ID in the response")
			}
			firstID = body.ID

			entries, err := h.Client.XRange(h.Ctx(), key, "-", "+", 0)
			if err != nil {
				t.Fatalf("XRange: %v", err)
			}
			if len(entries) != 1 {
				t.Fatalf("stream has %d entries, want 1", len(entries))
			}
			if entries[0].Fields["event"] != "created" || entries[0].Fields["user"] != "alice" {
				t.Errorf("fields = %v, want event=created user=alice", entries[0].Fields)
			}
		})

		t.Run("AddWithoutFieldsIsRejected", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/stream", map[string]any{"fields": map[string]string{}}).
				ExpectError(http.StatusBadRequest, "At least one field is required")
		})

		t.Run("EmptyFieldNameIsRejected", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/stream", map[string]any{"fields": map[string]string{"": "v"}}).
				ExpectError(http.StatusBadRequest, "Field name cannot be empty")
		})

		t.Run("EmptyFieldValueIsRejected", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/stream", map[string]any{"fields": map[string]string{"f": ""}}).
				ExpectError(http.StatusBadRequest, "Field value cannot be empty")
		})

		t.Run("Remove", func(t *testing.T) {
			h.Delete(h.KeyPath(key, "stream", firstID)).ExpectOK()
			length, err := h.Client.XLen(h.Ctx(), key)
			if err != nil {
				t.Fatalf("XLen: %v", err)
			}
			if length != 0 {
				t.Errorf("stream has %d entries, want 0", length)
			}
		})

		t.Run("RemoveMissingEntry", func(t *testing.T) {
			h.Delete(h.KeyPath(key, "stream", firstID)).
				ExpectError(http.StatusNotFound, "Entry not found")
		})

		t.Run("RemoveMalformedID", func(t *testing.T) {
			h.Delete(h.KeyPath(key, "stream", "not-an-id")).
				ExpectStatus(http.StatusInternalServerError)
		})
	})
}

func TestHyperLogLogEndpoints(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		const key = "crud:hll"

		t.Run("Add", func(t *testing.T) {
			for _, element := range []string{"alice", "bob", "carol"} {
				h.Post(h.KeyPath(key)+"/hll", map[string]any{"element": element}).ExpectOK()
			}
			count, err := h.Client.PFCount(h.Ctx(), key)
			if err != nil {
				t.Fatalf("PFCount: %v", err)
			}
			if count != 3 {
				t.Errorf("count = %d, want 3", count)
			}
		})

		t.Run("EmptyElementIsRejected", func(t *testing.T) {
			h.Post(h.KeyPath(key)+"/hll", map[string]any{"element": ""}).
				ExpectError(http.StatusBadRequest, "cannot be empty")
		})

		t.Run("CountIsVisibleThroughGetKey", func(t *testing.T) {
			resp := getKey(t, h, key)
			if resp.Type != "hyperloglog" {
				t.Errorf("type = %q, want hyperloglog", resp.Type)
			}
			if value := decodeValue[map[string]int64](t, resp); value["count"] != 3 {
				t.Errorf("count = %d, want 3", value["count"])
			}
		})
	})
}
