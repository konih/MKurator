package mqadmin

import "testing"

func TestChannelAuthNeedsUpdate(t *testing.T) {
	t.Parallel()
	if !ChannelAuthNeedsUpdate(ChannelAuthSpec{}, nil) {
		t.Fatal("nil observed should need update")
	}
	desired := ChannelAuthSpec{
		ChannelName: "CH1",
		RuleType:    ChannelAuthRuleTypeAddressMap,
		Address:     "*",
		UserSource:  "CHANNEL",
		CheckClient: "REQUIRED",
		Description: "test",
	}
	observed := &ChannelAuthState{
		ChannelName: "CH1",
		RuleType:    ChannelAuthRuleTypeAddressMap,
		Address:     "*",
		UserSource:  "channel",
		CheckClient: "required",
		Description: "test",
	}
	if ChannelAuthNeedsUpdate(desired, observed) {
		t.Fatal("expected no update when attributes match (case-insensitive enums)")
	}
	observed.CheckClient = "ASQMGR"
	if !ChannelAuthNeedsUpdate(desired, observed) {
		t.Fatal("expected update when check client drifts")
	}
}

func TestChannelAuthNeedsUpdateAddress(t *testing.T) {
	t.Parallel()
	desired := ChannelAuthSpec{
		ChannelName: "CH1",
		RuleType:    ChannelAuthRuleTypeAddressMap,
		Address:     "*",
	}
	observed := &ChannelAuthState{ChannelName: "CH1", RuleType: ChannelAuthRuleTypeAddressMap, Address: "127.0.0.1"}
	if !ChannelAuthNeedsUpdate(desired, observed) {
		t.Fatal("expected update when address drifts")
	}
}

func TestChannelAuthNeedsUpdateDescriptionEmptyObserved(t *testing.T) {
	t.Parallel()
	desired := ChannelAuthSpec{
		ChannelName: "CH1",
		RuleType:    ChannelAuthRuleTypeAddressMap,
		Address:     "*",
		Description: "e2e address map rule",
	}
	observed := &ChannelAuthState{
		ChannelName: "CH1",
		RuleType:    ChannelAuthRuleTypeAddressMap,
		Address:     "*",
	}
	if ChannelAuthNeedsUpdate(desired, observed) {
		t.Fatal("expected no update when DISPLAY omits DESCR (empty observed description)")
	}
}

func TestChannelAuthNeedsUpdateDescription(t *testing.T) {
	t.Parallel()
	desired := ChannelAuthSpec{
		ChannelName: "CH1",
		RuleType:    ChannelAuthRuleTypeAddressMap,
		Address:     "*",
		Description: "new",
	}
	observed := &ChannelAuthState{
		ChannelName: "CH1",
		RuleType:    ChannelAuthRuleTypeAddressMap,
		Address:     "*",
		Description: "old",
	}
	if !ChannelAuthNeedsUpdate(desired, observed) {
		t.Fatal("expected update when description drifts")
	}
}

func TestChannelAuthNeedsUpdateUserList(t *testing.T) {
	t.Parallel()
	desired := ChannelAuthSpec{
		ChannelName: "CH1",
		RuleType:    ChannelAuthRuleTypeBlockUser,
		UserList:    "nobody",
	}
	observed := &ChannelAuthState{
		ChannelName: "CH1",
		RuleType:    ChannelAuthRuleTypeBlockUser,
		UserList:    "nobody",
	}
	if ChannelAuthNeedsUpdate(desired, observed) {
		t.Fatal("expected no update when user list matches")
	}
	observed.UserList = "admin"
	if !ChannelAuthNeedsUpdate(desired, observed) {
		t.Fatal("expected update when user list drifts")
	}
}

func TestChannelAuthNeedsUpdateUserMap(t *testing.T) {
	t.Parallel()
	desired := ChannelAuthSpec{
		ChannelName: "CH1",
		RuleType:    ChannelAuthRuleTypeUserMap,
		ClientUser:  "johndoe",
		UserSource:  "MAP",
		McaUser:     "orders-app",
	}
	observed := &ChannelAuthState{
		ChannelName: "CH1",
		RuleType:    ChannelAuthRuleTypeUserMap,
		ClientUser:  "johndoe",
		UserSource:  "map",
		McaUser:     "orders-app",
		CheckClient: "ASQMGR",
	}
	if ChannelAuthNeedsUpdate(desired, observed) {
		t.Fatal("expected no update when USERMAP attributes match and CheckClient is MQ default")
	}
	observed.McaUser = "other"
	if !ChannelAuthNeedsUpdate(desired, observed) {
		t.Fatal("expected update when mcaUser drifts")
	}
}

func TestChannelAuthNeedsUpdateUserMapEmptyObservedClientUser(t *testing.T) {
	t.Parallel()
	desired := ChannelAuthSpec{
		ChannelName: "CH1",
		RuleType:    ChannelAuthRuleTypeUserMap,
		ClientUser:  "johndoe",
		UserSource:  "MAP",
		McaUser:     "orders-app",
	}
	observed := &ChannelAuthState{
		ChannelName: "CH1",
		RuleType:    ChannelAuthRuleTypeUserMap,
		UserSource:  "MAP",
		McaUser:     "orders-app",
		CheckClient: "ASQMGR",
	}
	if !ChannelAuthNeedsUpdate(desired, observed) {
		t.Fatal("expected update when DISPLAY omits CLNTUSER (empty observed clientUser)")
	}
}

func TestChannelAuthNeedsUpdateUserMapUserSourceChannel(t *testing.T) {
	t.Parallel()
	desired := ChannelAuthSpec{
		ChannelName: "CH1",
		RuleType:    ChannelAuthRuleTypeUserMap,
		ClientUser:  "johndoe",
		UserSource:  "CHANNEL",
	}
	observed := &ChannelAuthState{
		ChannelName: "CH1",
		RuleType:    ChannelAuthRuleTypeUserMap,
		ClientUser:  "johndoe",
		UserSource:  "channel",
		McaUser:     "legacy-mca",
		CheckClient: "ASQMGR",
	}
	if ChannelAuthNeedsUpdate(desired, observed) {
		t.Fatal("expected no update when USERSRC CHANNEL and desired mcaUser is unset")
	}
}

func TestAuthoritySetsEqualDifferentLengths(t *testing.T) {
	t.Parallel()
	if authoritySetsEqual([]string{"GET", "PUT"}, []string{"GET"}) {
		t.Fatal("expected unequal for different lengths")
	}
}

func TestNormalizeAuthoritySetDropsEmpty(t *testing.T) {
	t.Parallel()
	got := normalizeAuthoritySet([]string{" GET ", "", "put"})
	if len(got) != 2 || got[0] != "GET" || got[1] != "PUT" {
		t.Fatalf("normalizeAuthoritySet() = %v", got)
	}
}

func TestAuthorityNeedsUpdate(t *testing.T) {
	t.Parallel()
	if !AuthorityNeedsUpdate(AuthoritySpec{Authorities: []string{"GET"}}, nil) {
		t.Fatal("nil observed should need update")
	}
	desired := AuthoritySpec{
		Profile:     "APP.ORDERS",
		ObjectType:  AuthorityObjectTypeQueue,
		Principal:   "app",
		Authorities: []string{"GET", "PUT"},
	}
	observed := &AuthorityState{
		Profile:     "APP.ORDERS",
		ObjectType:  AuthorityObjectTypeQueue,
		Principal:   "app",
		Authorities: []string{"put", "get"},
	}
	if AuthorityNeedsUpdate(desired, observed) {
		t.Fatal("expected no update when authority sets match")
	}
	observed.Authorities = []string{"GET"}
	if !AuthorityNeedsUpdate(desired, observed) {
		t.Fatal("expected update when authorities drift")
	}
}
