package mocks

import "github.com/stretchr/testify/mock"

// FilterableContainer is a mock type for the FilterableContainer type.
type FilterableContainer struct {
	mock.Mock
}

// Enabled provides a mock function with given fields:.
func (_m *FilterableContainer) Enabled() (bool, bool) {
	ret := _m.Called()

	var result0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		result0 = rf()
	} else {
		result0 = ret.Get(0).(bool)
	}

	var result1 bool
	if rf, ok := ret.Get(1).(func() bool); ok {
		result1 = rf()
	} else {
		result1 = ret.Get(1).(bool)
	}

	return result0, result1
}

// IsWatchtower provides a mock function with given fields:.
func (_m *FilterableContainer) IsWatchtower() bool {
	ret := _m.Called()

	var result0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		result0 = rf()
	} else {
		result0 = ret.Get(0).(bool)
	}

	return result0
}

// Name provides a mock function with given fields:.
func (_m *FilterableContainer) Name() string {
	ret := _m.Called()

	var result0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		result0 = rf()
	} else {
		result0 = ret.Get(0).(string)
	}

	return result0
}

// Scope provides a mock function with given fields:.
func (_m *FilterableContainer) Scope() (string, bool) {
	ret := _m.Called()

	var result0 string

	if rf, ok := ret.Get(0).(func() string); ok {
		result0 = rf()
	} else {
		result0 = ret.Get(0).(string)
	}

	var result1 bool

	if rf, ok := ret.Get(1).(func() bool); ok {
		result1 = rf()
	} else {
		result1 = ret.Get(1).(bool)
	}

	return result0, result1
}

// ImageName provides a mock function with given fields:.
func (_m *FilterableContainer) ImageName() string {
	ret := _m.Called()

	var result0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		result0 = rf()
	} else {
		result0 = ret.Get(0).(string)
	}

	return result0
}

// GetLabel provides a mock function with given fields: key.
func (_m *FilterableContainer) GetLabel(key string) (string, bool) {
	ret := _m.Called(key)

	var result0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		result0 = rf(key)
	} else {
		result0 = ret.Get(0).(string)
	}

	var result1 bool
	if rf, ok := ret.Get(1).(func(string) bool); ok {
		result1 = rf(key)
	} else {
		result1 = ret.Get(1).(bool)
	}

	return result0, result1
}
