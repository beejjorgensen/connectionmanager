package main

import (
	"testing"
)

func userTest(t *testing.T, um *UserManager, name string, id string) {
	u, err := um.GetUserByID(id)

	if err != nil {
		t.Errorf("GetUserById: %v", err)
		return
	}

	if u.name != name {
		t.Errorf("u.name should be \"%s\", is \"%s\"", name, u.name)
	}

	if u.id != id {
		t.Errorf("u.id should be \"%s\", is \"%s\"", id, u.id)
	}

	pid := u.pubId

	u2, err := um.GetUserByPubID(pid)

	if err != nil {
		t.Errorf("GetUserByPubId: %v", err)
		return
	}

	if u.name != u2.name {
		t.Errorf("u.name does not match u2.name: \"%s\" \"%s\"", u.name, u2.name)
	}
	if u.id != u2.id {
		t.Errorf("u.id does not match u2.id: \"%s\" \"%s\"", u.id, u2.id)
	}
	if u.pubId != u2.pubId {
		t.Errorf("u.pubId does not match u2.pubId: \"%s\" \"%s\"", u.pubId, u2.pubId)
	}
}

func TestUsermanager(t *testing.T) {
	var err error

	um := NewUserManager()

	um.Start()

	_, err = um.AddUser("alpha", "idA")

	if err != nil {
		t.Errorf("error adding user alpha: %v", err)
		return
	}

	userTest(t, um, "idA", "alpha")

	_, err = um.AddUser("bravo", "idB")

	if err != nil {
		t.Errorf("error adding user beta: %v", err)
		return
	}

	userTest(t, um, "idA", "alpha")
	userTest(t, um, "idB", "bravo")
	userTest(t, um, "idA", "alpha")

	_, err = um.AddUser("charlie", "idC")

	if err != nil {
		t.Errorf("error adding user charlie: %v", err)
		return
	}

	userTest(t, um, "idC", "charlie")
	userTest(t, um, "idA", "alpha")
	userTest(t, um, "idB", "bravo")
	userTest(t, um, "idC", "charlie")
	userTest(t, um, "idB", "bravo")
	userTest(t, um, "idA", "alpha")
}
