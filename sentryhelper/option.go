package sentryhelper

import (
	"github.com/getsentry/sentry-go"
)

type Option interface {
	Apply(*config)
}

type Tag map[string]string

type Extra map[string]any

func NewExtra() Extra {
	return make(Extra)
}

func (e Extra) Set(key string, value any) {
	e[key] = value
}

type Contexts map[string]sentry.Context

func NewContexts() Contexts {
	return make(Contexts)
}

func (c Contexts) Set(key string, value sentry.Context) {
	c[key] = value
}

type config struct {
	Level           sentry.Level
	Breadcrumbs     []*sentry.Breadcrumb
	Attachments     []*sentry.Attachment
	Tag             Tag
	Contexts        Contexts
	Extra           Extra
	User            *sentry.User
	Fingerprint     []string
	EventProcessors []sentry.EventProcessor
}

type (
	OptionLevel           sentry.Level
	OptionBreadcrumbs     []*sentry.Breadcrumb
	OptionAttachments     []*sentry.Attachment
	OptionTag             Tag
	OptionContexts        Contexts
	OptionExtra           Extra
	OptionUser            sentry.User
	OptionFingerprint     []string
	OptionEventProcessors []sentry.EventProcessor
)

func (o OptionLevel) Apply(c *config) {
	c.Level = sentry.Level(o)
}

// WithLevel
// Sets the level of the event.
func WithLevel(level sentry.Level) OptionLevel {
	return OptionLevel(level)
}

func (o OptionBreadcrumbs) Apply(c *config) {
	c.Breadcrumbs = o
}

// WithBreadcrumbs
// Sets the breadcrumbs to be sent with the event.
func WithBreadcrumbs(bs ...sentry.Breadcrumb) OptionBreadcrumbs {
	opts := make([]*sentry.Breadcrumb, 0, len(bs))
	for _, b := range bs {
		opts = append(opts, &b)
	}
	return opts
}

func (o OptionAttachments) Apply(c *config) {
	c.Attachments = o
}

// WithAttachments
// Sets the attachments to be sent with the event.
func WithAttachments(bs ...sentry.Attachment) OptionAttachments {
	opts := make([]*sentry.Attachment, 0, len(bs))
	for _, b := range bs {
		opts = append(opts, &b)
	}
	return opts
}

func (o OptionTag) Apply(c *config) {
	c.Tag = Tag(o)
}

// WithTag
// Sets the tags to be sent with the event.
func WithTag(t Tag) OptionTag {
	return OptionTag(t)
}

func (o OptionContexts) Apply(c *config) {
	c.Contexts = Contexts(o)
}

// WithContexts
// Sets the contexts to be sent with the event.
func WithContexts(c Contexts) OptionContexts {
	return OptionContexts(c)
}

func (o OptionExtra) Apply(c *config) {
	c.Extra = Extra(o)
}

// WithExtra
// Sets the extra data to be sent with the event.
func WithExtra(extra Extra) OptionExtra {
	return OptionExtra(extra)
}

func (o OptionUser) Apply(c *config) {
	u := sentry.User(o)
	c.User = &u
}

// WithUser
// Sets the user data to be sent with the event.
func WithUser(extra sentry.User) OptionUser {
	return OptionUser(extra)
}

func (o OptionFingerprint) Apply(c *config) {
	c.Fingerprint = o
}

// WithFingerprint
// Sets the fingerprint for the event.
func WithFingerprint(fingerprint ...string) OptionFingerprint {
	return fingerprint
}

func (o OptionEventProcessors) Apply(c *config) {
	c.EventProcessors = o
}

// WithEventProcessors
// Sets the event processors for the event.
func WithEventProcessors(eventProcessors ...sentry.EventProcessor) OptionEventProcessors {
	return eventProcessors
}
