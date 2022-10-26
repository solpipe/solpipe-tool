package people

import (
	"encoding/json"
	"os"
)

type Group struct {
	Users map[uint64]*User `json:"users"`
}

func NewRandomGroup(count uint64) (*Group, error) {
	var err error
	g1 := new(Group)
	g1.Users = make(map[uint64]*User)
	for i := uint64(0); i < count; i++ {
		g1.Users[i], err = NewRandomUser(i)
		if err != nil {
			return nil, err
		}
	}
	return g1, nil
}

func (g1 *Group) Save(fp string) error {
	f, err := os.Create(fp)
	if err != nil {
		f, err = os.Open(fp)
		if err != nil {
			return err
		}
	}
	return json.NewEncoder(f).Encode(g1)
}

func LoadGroupFromFile(fp string) (*Group, error) {
	g1 := new(Group)
	f, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	err = json.NewDecoder(f).Decode(g1)
	if err != nil {
		return nil, err
	}
	return g1, nil
}
