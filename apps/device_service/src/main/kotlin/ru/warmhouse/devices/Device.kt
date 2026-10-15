package ru.warmhouse.devices

import java.time.Instant
import java.util.UUID

enum class DeviceStatus { ACTIVE, DISABLED }

enum class CommandSource { USER, SCENARIO, HEATING, LIGHTING, GATES, SURVEILLANCE }

data class Device(
    val id: UUID,
    val name: String,
    val typeCode: String,
    val location: String?,
    val status: DeviceStatus,
    val legacySensorId: Int?,
    val createdAt: Instant,
    val updatedAt: Instant,
)

data class CreateDeviceRequest(
    val name: String?,
    val typeCode: String?,
    val location: String?,
)

data class SendCommandRequest(
    val name: String?,
    val params: Map<String, Any?> = emptyMap(),
    val source: CommandSource?,
)

/** Command resource returned by the REST API. */
data class Command(
    val id: UUID,
    val deviceId: UUID,
    val name: String,
    val params: Map<String, Any?>,
    val source: CommandSource,
    val status: String,
    val createdAt: Instant,
    val completedAt: Instant? = null,
    val error: String? = null,
)

/** DeviceCommand message published to the `device.commands` exchange. */
data class DeviceCommandMessage(
    val commandId: UUID,
    val deviceId: UUID,
    val deviceType: String,
    val name: String,
    val params: Map<String, Any?>,
    val source: CommandSource,
    val issuedAt: Instant,
    val expiresAt: Instant,
)

/** Event published by the monolith to the `legacy.sensors` exchange. */
data class LegacySensorEvent(
    val eventType: String,
    val deviceId: UUID,
    val legacySensorId: Int,
    val name: String,
    val type: String,
    val location: String?,
)
