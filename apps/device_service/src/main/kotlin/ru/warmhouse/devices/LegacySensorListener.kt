package ru.warmhouse.devices

import org.slf4j.LoggerFactory
import org.springframework.amqp.rabbit.annotation.RabbitListener
import org.springframework.stereotype.Component

/** Mirrors sensors managed by the monolith into the device registry. */
@Component
class LegacySensorListener(private val repository: DeviceRepository) {

    private val log = LoggerFactory.getLogger(javaClass)

    @RabbitListener(queues = [RabbitConfig.LEGACY_SENSORS_QUEUE])
    fun onSensorEvent(event: LegacySensorEvent) {
        log.info("Legacy sensor {} {} (device {})", event.legacySensorId, event.eventType, event.deviceId)
        when (event.eventType) {
            "created", "updated" -> repository.upsertLegacy(event, toDeviceType(event.type))
            "deleted" -> repository.disable(event.deviceId)
            else -> log.warn("Unknown legacy sensor event type: {}", event.eventType)
        }
    }

    private fun toDeviceType(legacyType: String) = if (legacyType == "temperature") "temp_sensor" else legacyType
}
