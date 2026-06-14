package mqadmin

// ChannelAuthRuleType mirrors CHLAUTH TYPE values.
type ChannelAuthRuleType string

const (
	ChannelAuthRuleTypeAddressMap ChannelAuthRuleType = "ADDRESSMAP"
	ChannelAuthRuleTypeBlockUser  ChannelAuthRuleType = "BLOCKUSER"
	ChannelAuthRuleTypeUserMap    ChannelAuthRuleType = "USERMAP"
	ChannelAuthRuleTypeSSLPeerMap ChannelAuthRuleType = "SSLPEERMAP"
	ChannelAuthRuleTypeQMGRMap    ChannelAuthRuleType = "QMGRMAP"
	ChannelAuthRuleTypeBlockAddr  ChannelAuthRuleType = "BLOCKADDR"
)

// ChannelAuthSpec is the domain shape for SET CHLAUTH.
type ChannelAuthSpec struct {
	ChannelName        string
	RuleType           ChannelAuthRuleType
	Address            string
	UserList           string
	ClientUser         string
	SSLPeerName        string
	RemoteQueueManager string
	McaUser            string
	UserSource         string
	CheckClient        string
	Description        string
}

// AuthorityObjectType mirrors AUTHREC OBJTYPE values.
type AuthorityObjectType string

const (
	AuthorityObjectTypeQueue     AuthorityObjectType = "QUEUE"
	AuthorityObjectTypeChannel   AuthorityObjectType = "CHANNEL"
	AuthorityObjectTypeTopic     AuthorityObjectType = "TOPIC"
	AuthorityObjectTypeQMGR      AuthorityObjectType = "QMGR"
	AuthorityObjectTypeNamespace AuthorityObjectType = "NAMESPAC"
	AuthorityObjectTypeProcess   AuthorityObjectType = "PROCESS"
	AuthorityObjectTypeNList     AuthorityObjectType = "NLIST"
)

// AuthoritySpec is the domain shape for SET AUTHREC.
type AuthoritySpec struct {
	Profile     string
	ObjectType  AuthorityObjectType
	Principal   string
	Group       string
	Authorities []string
}

// ChannelAuthState is the observed attributes of a CHLAUTH rule.
type ChannelAuthState struct {
	ChannelName        string
	RuleType           ChannelAuthRuleType
	Address            string
	UserList           string
	ClientUser         string
	SSLPeerName        string
	RemoteQueueManager string
	McaUser            string
	UserSource         string
	CheckClient        string
	Description        string
}

// AuthorityState is the observed OAM authorities for a profile/principal or group.
type AuthorityState struct {
	Profile     string
	ObjectType  AuthorityObjectType
	Principal   string
	Group       string
	Authorities []string
}
