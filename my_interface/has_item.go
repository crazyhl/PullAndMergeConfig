package my_interface

type HasItemInterface interface {
	HasItem(name string) (interface{}, error)
}

// 通用的获取 items 方法
func GetItems(items HasItemInterface, name string) (interface{}, error) {
	return items.HasItem(name)
}
