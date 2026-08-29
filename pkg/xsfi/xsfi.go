// "accessify" - URL namespace
package xsfi

import (
	"path"
)

var Root = Namespace{"xs.fi"}

type Namespace struct {
	Prefix string
}

type ID string

func (i ID) URL() string {
	return "https://" + string(i)
}

func (n Namespace) ID(id string) ID {
	return ID(path.Join(n.Prefix, id))
}

func (n Namespace) Sub(id string) Namespace {
	return Namespace{path.Join(n.Prefix, id)}
}
