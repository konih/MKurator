package mqadmin

// QueueType mirrors the CRD queue kind.
type QueueType string

const (
	QueueTypeLocal  QueueType = "local"
	QueueTypeAlias  QueueType = "alias"
	QueueTypeRemote QueueType = "remote"
)

// NormalizeQueueType returns local when empty.
func NormalizeQueueType(t QueueType) QueueType {
	if t == "" {
		return QueueTypeLocal
	}
	return t
}
