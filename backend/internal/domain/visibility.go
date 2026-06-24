package domain

type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityPrivate  Visibility = "private"
	VisibilityUnlisted Visibility = "unlisted"
)

func (v Visibility) Valid() bool {
	switch v {
	case VisibilityPublic, VisibilityPrivate, VisibilityUnlisted:
		return true
	default:
		return false
	}
}
