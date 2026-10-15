package ru.warmhouse.devices

import org.springframework.amqp.core.MessageDeliveryMode
import org.springframework.amqp.rabbit.core.RabbitTemplate
import org.springframework.stereotype.Component
import java.time.Duration

@Component
class CommandPublisher(private val rabbitTemplate: RabbitTemplate) {

    companion object {
        /** A command not delivered within this time is dropped: executing it late is worse than not at all. */
        val COMMAND_TTL: Duration = Duration.ofSeconds(30)
    }

    fun publish(command: DeviceCommandMessage) {
        rabbitTemplate.convertAndSend(
            RabbitConfig.DEVICE_COMMANDS_EXCHANGE,
            "${command.deviceType}.${command.deviceId}",
            command,
        ) { message ->
            message.messageProperties.apply {
                messageId = command.commandId.toString()
                type = "device.command"
                deliveryMode = MessageDeliveryMode.PERSISTENT
                expiration = COMMAND_TTL.toMillis().toString()
            }
            message
        }
    }
}
