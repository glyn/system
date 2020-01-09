/*
Copyright 2020 the original author or authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package builders

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/projectriff/system/pkg/apis"
)

type condition struct {
	target *apis.Condition
}

func Condition(seed ...apis.Condition) *condition {
	var target *apis.Condition
	switch len(seed) {
	case 0:
		target = &apis.Condition{}
	case 1:
		target = &seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &condition{
		target: target,
	}
}

func (b *condition) deepCopy() *condition {
	return Condition(*b.target.DeepCopy())
}

func (b *condition) Build() apis.Condition {
	return *b.deepCopy().target
}

func (b *condition) Mutate(m func(*apis.Condition)) *condition {
	b = b.deepCopy()
	m(b.target)
	return b
}

func (b *condition) Type(t apis.ConditionType) *condition {
	return b.Mutate(func(c *apis.Condition) {
		c.Type = t
	})
}

func (b *condition) Unknown() *condition {
	return b.Mutate(func(c *apis.Condition) {
		c.Status = corev1.ConditionUnknown
	})
}

func (b *condition) True() *condition {
	return b.Mutate(func(c *apis.Condition) {
		c.Status = corev1.ConditionTrue
		c.Reason = ""
		c.Message = ""
	})
}

func (b *condition) False() *condition {
	return b.Mutate(func(c *apis.Condition) {
		c.Status = corev1.ConditionFalse
	})
}

func (b *condition) Reason(reason, message string) *condition {
	return b.Mutate(func(c *apis.Condition) {
		c.Reason = reason
		c.Message = message
	})
}

func (b *condition) Info() *condition {
	return b.Mutate(func(c *apis.Condition) {
		c.Severity = apis.ConditionSeverityInfo
	})
}

func (b *condition) Warning() *condition {
	return b.Mutate(func(c *apis.Condition) {
		c.Severity = apis.ConditionSeverityWarning
	})
}

func (b *condition) Error() *condition {
	return b.Mutate(func(c *apis.Condition) {
		c.Severity = apis.ConditionSeverityError
	})
}
