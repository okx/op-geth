// For X Layer

package common

import "slices"

type OrderedList[T any] struct {
	list        []T
	isOrdered   bool
	compareFunc func(a, b T) int
}

func (l *OrderedList[T]) Add(item T) {
	l.isOrdered = false
	l.list = append(l.list, item)
}

func (l *OrderedList[T]) Sort() {
	slices.SortFunc(l.list, l.compareFunc)
	l.isOrdered = true
}

func (l *OrderedList[T]) containsBinarySearch(item T) bool {
	upper := len(l.list)
	lower := 0
	for lower < upper {
		mid := (upper + lower) / 2
		cmp := l.compareFunc(item, l.list[mid])
		if cmp == 0 {
			return true
		}
		if cmp > 0 {
			lower = mid + 1
		} else {
			upper = mid
		}
	}
	return false
}

func (l *OrderedList[T]) containsLinear(item T) bool {
	for _, i := range l.list {
		if l.compareFunc(i, item) == 0 {
			return true
		}
	}
	return false
}

func (l *OrderedList[T]) Contains(item T) bool {
	if !l.isOrdered {
		l.containsLinear(item)
	}
	return l.containsBinarySearch(item)
}

func (l *OrderedList[T]) Size() int {
	return len(l.list)
}

func (l *OrderedList[T]) Items() []T {
	return l.list
}

func CompareAddressess(a, b Address) int {
	for i := 0; i < len(a); i++ {
		if a[i] < b[i] {
			return int(a[i]) - int(b[i])
		}
		if a[i] > b[i] {
			return int(a[i]) - int(b[i])
		}
	}
	return 0
}

func NewOrderedListOfAddresses(size int) *OrderedList[Address] {
	return &OrderedList[Address]{
		list:        make([]Address, 0, size),
		isOrdered:   false,
		compareFunc: CompareAddressess,
	}
}

func ToListOfString(list *OrderedList[Address]) []string {
	strs := make([]string, list.Size())
	for i, addr := range list.Items() {
		strs[i] = addr.String()
	}
	return strs
}
