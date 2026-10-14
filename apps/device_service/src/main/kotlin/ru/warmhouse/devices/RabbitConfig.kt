package ru.warmhouse.devices

import com.fasterxml.jackson.databind.ObjectMapper
import org.springframework.amqp.core.Binding
import org.springframework.amqp.core.BindingBuilder
import org.springframework.amqp.core.Queue
import org.springframework.amqp.core.TopicExchange
import org.springframework.amqp.support.converter.Jackson2JsonMessageConverter
import org.springframework.amqp.support.converter.MessageConverter
import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration

@Configuration
class RabbitConfig {

    companion object {
        const val DEVICE_COMMANDS_EXCHANGE = "device.commands"
        const val LEGACY_SENSORS_EXCHANGE = "legacy.sensors"
        const val LEGACY_SENSORS_QUEUE = "devices.legacy-sensors"
    }

    @Bean
    fun deviceCommandsExchange() = TopicExchange(DEVICE_COMMANDS_EXCHANGE, true, false)

    @Bean
    fun legacySensorsExchange() = TopicExchange(LEGACY_SENSORS_EXCHANGE, true, false)

    @Bean
    fun legacySensorsQueue() = Queue(LEGACY_SENSORS_QUEUE, true)

    @Bean
    fun legacySensorsBinding(legacySensorsQueue: Queue, legacySensorsExchange: TopicExchange): Binding =
        BindingBuilder.bind(legacySensorsQueue).to(legacySensorsExchange).with("sensor.*")

    @Bean
    fun messageConverter(objectMapper: ObjectMapper): MessageConverter = Jackson2JsonMessageConverter(objectMapper)
}
