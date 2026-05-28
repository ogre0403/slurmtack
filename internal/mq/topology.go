package mq

const (
	ExchangeName          = "gpu-switch.events"
	RequestedQueueName    = "gpu-switch.requested"
	NodeSelectedQueueName = "gpu-switch.node-selected"
	AllocationQueueName   = "gpu-switch.allocation"
	DrainedQueueName      = "gpu-switch.drained"

	RequestedRoutingKey    = "execution.requested"
	NodeSelectedRoutingKey = "execution.node_selected"
	AllocationRoutingKey   = "execution.allocation"
	DrainedRoutingKey      = "execution.drained"
)

func DeclareTopology(c *Connection) error {
	ch := c.Channel()

	err := ch.ExchangeDeclare(
		ExchangeName,
		"topic",
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,
	)
	if err != nil {
		return err
	}

	_, err = ch.QueueDeclare(
		RequestedQueueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		return err
	}

	_, err = ch.QueueDeclare(
		NodeSelectedQueueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		return err
	}

	_, err = ch.QueueDeclare(
		AllocationQueueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		return err
	}

	_, err = ch.QueueDeclare(
		DrainedQueueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		return err
	}

	err = ch.QueueBind(RequestedQueueName, RequestedRoutingKey, ExchangeName, false, nil)
	if err != nil {
		return err
	}

	err = ch.QueueBind(NodeSelectedQueueName, NodeSelectedRoutingKey, ExchangeName, false, nil)
	if err != nil {
		return err
	}

	err = ch.QueueBind(AllocationQueueName, AllocationRoutingKey, ExchangeName, false, nil)
	if err != nil {
		return err
	}

	err = ch.QueueBind(DrainedQueueName, DrainedRoutingKey, ExchangeName, false, nil)
	if err != nil {
		return err
	}

	return nil
}
