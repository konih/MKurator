package mqadmin

import (
	"sort"
	"strings"
)

// ChannelAuthNeedsUpdate reports whether desired CHLAUTH attributes differ from observed.
func ChannelAuthNeedsUpdate(desired ChannelAuthSpec, observed *ChannelAuthState) bool {
	if observed == nil {
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(desired.Address), strings.TrimSpace(observed.Address)) {
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(desired.UserList), strings.TrimSpace(observed.UserList)) {
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(desired.ClientUser), strings.TrimSpace(observed.ClientUser)) {
		return true
	}
	// Only compare SET-managed fields when desired is non-empty. USERMAP DISPLAY often
	// returns CHCKCLNT(ASQMGR) and may surface MCAUSER even when USERSRC is CHANNEL;
	// empty desired means the operator does not manage that attribute on SET.
	if strings.TrimSpace(desired.McaUser) != "" &&
		!strings.EqualFold(strings.TrimSpace(desired.McaUser), strings.TrimSpace(observed.McaUser)) {
		return true
	}
	if strings.TrimSpace(desired.UserSource) != "" &&
		!strings.EqualFold(strings.TrimSpace(desired.UserSource), strings.TrimSpace(observed.UserSource)) {
		return true
	}
	if strings.TrimSpace(desired.CheckClient) != "" &&
		!strings.EqualFold(strings.TrimSpace(desired.CheckClient), strings.TrimSpace(observed.CheckClient)) {
		return true
	}
	// IBM MQ DISPLAY CHLAUTH text often omits DESCR; treat empty observed as unknown.
	if obs := strings.TrimSpace(observed.Description); obs != "" &&
		strings.TrimSpace(desired.Description) != obs {
		return true
	}
	return false
}

// AuthorityNeedsUpdate reports whether desired OAM authorities differ from observed.
func AuthorityNeedsUpdate(desired AuthoritySpec, observed *AuthorityState) bool {
	if observed == nil {
		return true
	}
	return !authoritySetsEqual(desired.Authorities, observed.Authorities)
}

func authoritySetsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	na := normalizeAuthoritySet(a)
	nb := normalizeAuthoritySet(b)
	for i := range na {
		if na[i] != nb[i] {
			return false
		}
	}
	return true
}

func normalizeAuthoritySet(authorities []string) []string {
	out := make([]string, 0, len(authorities))
	for _, auth := range authorities {
		auth = strings.ToUpper(strings.TrimSpace(auth))
		if auth != "" {
			out = append(out, auth)
		}
	}
	sort.Strings(out)
	return out
}
