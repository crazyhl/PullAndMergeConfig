package my_interface

type HasItemInterface interface {
	HasItem(name string) (interface{}, error)
}
