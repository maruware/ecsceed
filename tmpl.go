package ecsceed

import (
	"bytes"
	"encoding/json"
	"text/template"
)

func loadAndMatchTmpl(file string, params Params, dst interface{}) error {
	tpl, err := template.ParseFiles(file)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(nil)
	err = tpl.Execute(buf, params)
	if err != nil {
		return err
	}
	d := json.NewDecoder(buf)
	if err := d.Decode(dst); err != nil {
		return err
	}

	return nil
}
