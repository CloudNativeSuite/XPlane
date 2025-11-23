package gtm

import "fmt"

// NodeStatusFromString parses a status string into a NodeStatus value.
func NodeStatusFromString(status string) (NodeStatus, error) {
	switch status {
	case "", string(NodeStatusUp):
		return NodeStatusUp, nil
	case string(NodeStatusDown):
		return NodeStatusDown, nil
	case string(NodeStatusDrain):
		return NodeStatusDrain, nil
	default:
		return "", fmt.Errorf("unknown node status %q", status)
	}
}
