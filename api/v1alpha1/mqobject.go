package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// MQObject is implemented by workload CRs reconciled against IBM MQ (Queue, Topic,
// Channel, ChannelAuthRule, AuthorityRecord). Controllers use it to patch status
// and resolve connection references without per-kind type switches.
type MQObject interface {
	GetMQConditions() *[]metav1.Condition
	GetMQStatusFields() *MQObjectStatusFields
	GetStatusObservedGeneration() *int64
	SetStatusObservedGeneration(int64)
	ConnectionRefName() string
}

func (q *Queue) GetMQConditions() *[]metav1.Condition     { return &q.Status.Conditions }
func (q *Queue) GetMQStatusFields() *MQObjectStatusFields { return &q.Status.MQObjectStatusFields }
func (q *Queue) GetStatusObservedGeneration() *int64      { return &q.Status.ObservedGeneration }
func (q *Queue) SetStatusObservedGeneration(g int64)      { q.Status.ObservedGeneration = g }
func (q *Queue) ConnectionRefName() string                { return q.Spec.ConnectionRef.Name }

func (t *Topic) GetMQConditions() *[]metav1.Condition     { return &t.Status.Conditions }
func (t *Topic) GetMQStatusFields() *MQObjectStatusFields { return &t.Status.MQObjectStatusFields }
func (t *Topic) GetStatusObservedGeneration() *int64      { return &t.Status.ObservedGeneration }
func (t *Topic) SetStatusObservedGeneration(g int64)      { t.Status.ObservedGeneration = g }
func (t *Topic) ConnectionRefName() string                { return t.Spec.ConnectionRef.Name }

func (c *Channel) GetMQConditions() *[]metav1.Condition     { return &c.Status.Conditions }
func (c *Channel) GetMQStatusFields() *MQObjectStatusFields { return &c.Status.MQObjectStatusFields }
func (c *Channel) GetStatusObservedGeneration() *int64      { return &c.Status.ObservedGeneration }
func (c *Channel) SetStatusObservedGeneration(g int64)      { c.Status.ObservedGeneration = g }
func (c *Channel) ConnectionRefName() string                { return c.Spec.ConnectionRef.Name }

func (r *ChannelAuthRule) GetMQConditions() *[]metav1.Condition { return &r.Status.Conditions }
func (r *ChannelAuthRule) GetMQStatusFields() *MQObjectStatusFields {
	return &r.Status.MQObjectStatusFields
}
func (r *ChannelAuthRule) GetStatusObservedGeneration() *int64 { return &r.Status.ObservedGeneration }
func (r *ChannelAuthRule) SetStatusObservedGeneration(g int64) { r.Status.ObservedGeneration = g }
func (r *ChannelAuthRule) ConnectionRefName() string           { return r.Spec.ConnectionRef.Name }

func (a *AuthorityRecord) GetMQConditions() *[]metav1.Condition { return &a.Status.Conditions }
func (a *AuthorityRecord) GetMQStatusFields() *MQObjectStatusFields {
	return &a.Status.MQObjectStatusFields
}
func (a *AuthorityRecord) GetStatusObservedGeneration() *int64 { return &a.Status.ObservedGeneration }
func (a *AuthorityRecord) SetStatusObservedGeneration(g int64) { a.Status.ObservedGeneration = g }
func (a *AuthorityRecord) ConnectionRefName() string           { return a.Spec.ConnectionRef.Name }
