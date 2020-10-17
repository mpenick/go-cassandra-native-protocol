package codec

import (
	"errors"
	"fmt"
	"go-cassandra-native-protocol/cassandraprotocol"
	"go-cassandra-native-protocol/cassandraprotocol/message"
	"go-cassandra-native-protocol/cassandraprotocol/primitives"
)

type EventCodec struct{}

func (c EventCodec) Encode(msg message.Message, dest []byte, version cassandraprotocol.ProtocolVersion) error {
	event := msg.(*message.Event)
	var err error
	dest, err = primitives.WriteString(event.Type, dest)
	if err != nil {
		return err
	}
	switch event.Type {
	case cassandraprotocol.EventTypeSchemaChange:
		sce, ok := msg.(*message.SchemaChangeEvent)
		if !ok {
			return errors.New(fmt.Sprintf("expected SchemaChangeEvent struct, got %T", sce))
		}
		dest, err = primitives.WriteString(sce.ChangeType, dest)
		if err != nil {
			return fmt.Errorf("cannot write SchemaChangeEvent.ChangeType: %w", err)
		}
		dest, err = primitives.WriteString(sce.Target, dest)
		if err != nil {
			return fmt.Errorf("cannot write SchemaChangeEvent.Target: %w", err)
		}
		dest, err = primitives.WriteString(sce.Keyspace, dest)
		if err != nil {
			return fmt.Errorf("cannot write SchemaChangeEvent.Keyspace: %w", err)
		}
		switch sce.Target {
		case cassandraprotocol.SchemaChangeTargetKeyspace:
		case cassandraprotocol.SchemaChangeTargetTable:
			fallthrough
		case cassandraprotocol.SchemaChangeTargetType:
			dest, err = primitives.WriteString(sce.Object, dest)
			if err != nil {
				return fmt.Errorf("cannot write SchemaChangeEvent.Object: %w", err)
			}
		case cassandraprotocol.SchemaChangeTargetAggregate:
			fallthrough
		case cassandraprotocol.SchemaChangeTargetFunction:
			if version < cassandraprotocol.ProtocolVersion4 {
				return errors.New(fmt.Sprintf("%s schema change events are not supported in protocol version %d", sce.Target, version))
			}
			dest, err = primitives.WriteString(sce.Object, dest)
			if err != nil {
				return fmt.Errorf("cannot write SchemaChangeEvent.Object: %w", err)
			}
			dest, err = primitives.WriteStringList(sce.Arguments, dest)
			if err != nil {
				return fmt.Errorf("cannot write SchemaChangeEvent.Arguments: %w", err)
			}
		default:
			return errors.New(fmt.Sprintf("unknown schema change target: " + sce.Target))
		}
		return nil
	case cassandraprotocol.EventTypeStatusChange:
		sce, ok := msg.(*message.StatusChangeEvent)
		if !ok {
			return errors.New(fmt.Sprintf("expected StatusChangeEvent struct, got %T", sce))
		}
		dest, err = primitives.WriteString(sce.ChangeType, dest)
		if err != nil {
			return fmt.Errorf("cannot write StatusChangeEvent.ChangeType: %w", err)
		}
		dest, err = primitives.WriteInet(sce.Address, dest)
		if err != nil {
			return fmt.Errorf("cannot write StatusChangeEvent.Address: %w", err)
		}
		return nil
	case cassandraprotocol.EventTypeTopologyChange:
		tce, ok := msg.(*message.TopologyChangeEvent)
		if !ok {
			return errors.New(fmt.Sprintf("expected TopologyChangeEvent struct, got %T", tce))
		}
		dest, err = primitives.WriteString(tce.ChangeType, dest)
		if err != nil {
			return fmt.Errorf("cannot write TopologyChangeEvent.ChangeType: %w", err)
		}
		dest, err = primitives.WriteInet(tce.Address, dest)
		if err != nil {
			return fmt.Errorf("cannot write TopologyChangeEvent.Address: %w", err)
		}
		return nil
	}
	return errors.New("unknown event type: " + event.Type)
}

func (c EventCodec) EncodedSize(msg message.Message, version cassandraprotocol.ProtocolVersion) (int, error) {
	event := msg.(*message.Event)
	size := primitives.LengthOfString(event.Type)
	switch event.Type {
	case cassandraprotocol.EventTypeSchemaChange:
		sce, ok := msg.(*message.SchemaChangeEvent)
		if !ok {
			return -1, errors.New(fmt.Sprintf("expected SchemaChangeEvent struct, got %T", sce))
		}
		size += primitives.LengthOfString(sce.ChangeType)
		size += primitives.LengthOfString(sce.Target)
		size += primitives.LengthOfString(sce.Keyspace)
		switch sce.Target {
		case cassandraprotocol.SchemaChangeTargetKeyspace:
		case cassandraprotocol.SchemaChangeTargetTable:
			fallthrough
		case cassandraprotocol.SchemaChangeTargetType:
			size += primitives.LengthOfString(sce.Object)
			break
		case cassandraprotocol.SchemaChangeTargetAggregate:
			fallthrough
		case cassandraprotocol.SchemaChangeTargetFunction:
			if version < cassandraprotocol.ProtocolVersion4 {
				return -1, errors.New(fmt.Sprintf("%s schema change events are not supported in protocol version %d", sce.Target, version))
			}
			size += primitives.LengthOfString(sce.Object)
			size += primitives.LengthOfStringList(sce.Arguments)
		default:
			return -1, errors.New(fmt.Sprintf("unknown schema change target: " + sce.Target))
		}
		return size, nil
	case cassandraprotocol.EventTypeStatusChange:
		sce, ok := msg.(*message.StatusChangeEvent)
		if !ok {
			return -1, errors.New(fmt.Sprintf("expected StatusChangeEvent struct, got %T", sce))
		}
		size += primitives.LengthOfString(sce.ChangeType)
		inetLength, err := primitives.LengthOfInet(sce.Address)
		if err != nil {
			return -1, fmt.Errorf("cannot compute length of StatusChangeEvent.Address: %w", err)
		}
		size += inetLength
		return size, nil
	case cassandraprotocol.EventTypeTopologyChange:
		tce, ok := msg.(*message.TopologyChangeEvent)
		if !ok {
			return -1, errors.New(fmt.Sprintf("expected TopologyChangeEvent struct, got %T", tce))
		}
		size += primitives.LengthOfString(tce.ChangeType)
		inetLength, err := primitives.LengthOfInet(tce.Address)
		if err != nil {
			return -1, fmt.Errorf("cannot compute length of TopologyChangeEvent.Address: %w", err)
		}
		size += inetLength
		return size, nil
	}
	return -1, errors.New("unknown event type: " + event.Type)
}

func (c EventCodec) Decode(source []byte, version cassandraprotocol.ProtocolVersion) (message.Message, error) {
	eventType, _, err := primitives.ReadString(source)
	if err != nil {
		return nil, err
	}
	switch eventType {
	case cassandraprotocol.EventTypeSchemaChange:
		sce := &message.SchemaChangeEvent{Event: message.Event{Type: eventType}}
		sce.ChangeType, source, err = primitives.ReadString(source)
		if err != nil {
			return nil, fmt.Errorf("cannot read SchemaChangeEvent.ChangeType: %w", err)
		}
		sce.Target, source, err = primitives.ReadString(source)
		if err != nil {
			return nil, fmt.Errorf("cannot read SchemaChangeEvent.Target: %w", err)
		}
		sce.Keyspace, source, err = primitives.ReadString(source)
		if err != nil {
			return nil, fmt.Errorf("cannot read SchemaChangeEvent.Keyspace: %w", err)
		}
		switch sce.Target {
		case cassandraprotocol.SchemaChangeTargetKeyspace:
		case cassandraprotocol.SchemaChangeTargetTable:
			fallthrough
		case cassandraprotocol.SchemaChangeTargetType:
			sce.Object, source, err = primitives.ReadString(source)
			if err != nil {
				return nil, fmt.Errorf("cannot read SchemaChangeEvent.Object: %w", err)
			}
		case cassandraprotocol.SchemaChangeTargetAggregate:
			fallthrough
		case cassandraprotocol.SchemaChangeTargetFunction:
			if version < cassandraprotocol.ProtocolVersion4 {
				return nil, errors.New(fmt.Sprintf("%s schema change events are not supported in protocol version %d", sce.Target, version))
			}
			sce.Object, source, err = primitives.ReadString(source)
			if err != nil {
				return nil, fmt.Errorf("cannot read SchemaChangeEvent.Object: %w", err)
			}
			sce.Arguments, source, err = primitives.ReadStringList(source)
			if err != nil {
				return nil, fmt.Errorf("cannot read SchemaChangeEvent.Arguments: %w", err)
			}
		default:
			return nil, errors.New(fmt.Sprintf("unknown schema change target: " + sce.Target))
		}
		return sce, nil
	case cassandraprotocol.EventTypeStatusChange:
		sce := &message.StatusChangeEvent{Event: message.Event{Type: eventType}}
		sce.ChangeType, source, err = primitives.ReadString(source)
		if err != nil {
			return nil, fmt.Errorf("cannot read StatusChangeEvent.ChangeType: %w", err)
		}
		sce.Address, source, err = primitives.ReadInet(source)
		if err != nil {
			return nil, fmt.Errorf("cannot read StatusChangeEvent.Address: %w", err)
		}
		return sce, nil
	case cassandraprotocol.EventTypeTopologyChange:
		tce := &message.TopologyChangeEvent{Event: message.Event{Type: eventType}}
		tce.ChangeType, source, err = primitives.ReadString(source)
		if err != nil {
			return nil, fmt.Errorf("cannot read TopologyChangeEvent.ChangeType: %w", err)
		}
		tce.Address, source, err = primitives.ReadInet(source)
		if err != nil {
			return nil, fmt.Errorf("cannot read TopologyChangeEvent.Address: %w", err)
		}
		return tce, nil
	}
	return nil, errors.New("unknown event type: " + eventType)
}
