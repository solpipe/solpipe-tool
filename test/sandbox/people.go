package sandbox

import ppl "github.com/solpipe/solpipe-tool/test/people"

func (e1 *Sandbox) fp_people() string {
	return e1.baseDir + "/people.json"
}

func (e1 *Sandbox) loadPeople() error {
	var err error
	e1.People, err = ppl.LoadGroupFromFile(e1.fp_people())
	if err != nil {
		return err
	}
	return nil
}

func (e1 *Sandbox) PrefundPeople(amount uint) {

}
