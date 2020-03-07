package model

// 自定义集合
type MySet struct {
	m map[interface{}]struct{}
}

// 添加
func (set *MySet) Add(key interface{}) {
	set.m[key] = struct{}{}
}

// 是否包含
func (set *MySet) Contains(key interface{}) bool {
	_, ok := set.m[key]

	return ok
}

// 移除
func (set *MySet) Remove(key interface{}) {
	exist := set.Contains(key)

	if exist {
		delete(set.m, key)
	}
}

// 长度
func (set *MySet) Size() int {
	return len(set.m)
}

// 清理
func (set *MySet) Clear() {
	set.m = make(map[interface{}]struct{})
}
