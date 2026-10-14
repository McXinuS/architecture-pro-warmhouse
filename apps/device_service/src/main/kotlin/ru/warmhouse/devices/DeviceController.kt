package ru.warmhouse.devices

import org.springframework.http.HttpStatus
import org.springframework.http.ResponseEntity
import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.PathVariable
import org.springframework.web.bind.annotation.PostMapping
import org.springframework.web.bind.annotation.RequestBody
import org.springframework.web.bind.annotation.RequestMapping
import org.springframework.web.bind.annotation.RequestParam
import org.springframework.web.bind.annotation.RestController
import org.springframework.web.server.ResponseStatusException
import java.net.URI
import java.time.Instant
import java.util.UUID

@RestController
@RequestMapping("/api/v1/devices")
class DeviceController(
    private val repository: DeviceRepository,
    private val commandPublisher: CommandPublisher,
) {

    @GetMapping
    fun list(@RequestParam(required = false) type: String?): List<Device> = repository.findAll(type)

    @GetMapping("/{id}")
    fun get(@PathVariable id: UUID): Device = findDevice(id)

    @PostMapping
    fun create(@RequestBody request: CreateDeviceRequest): ResponseEntity<Device> {
        if (request.name.isNullOrBlank() || request.typeCode.isNullOrBlank()) {
            throw ResponseStatusException(HttpStatus.BAD_REQUEST, "Поля name и typeCode обязательны")
        }
        val device = repository.create(UUID.randomUUID(), request.name, request.typeCode, request.location)
        return ResponseEntity.created(URI.create("/api/v1/devices/${device.id}")).body(device)
    }

    @PostMapping("/{id}/commands")
    fun sendCommand(@PathVariable id: UUID, @RequestBody request: SendCommandRequest): ResponseEntity<Command> {
        val device = findDevice(id)
        if (request.name.isNullOrBlank() || request.source == null) {
            throw ResponseStatusException(HttpStatus.BAD_REQUEST, "Поля name и source обязательны")
        }
        if (device.status == DeviceStatus.DISABLED) {
            throw ResponseStatusException(HttpStatus.CONFLICT, "Устройство ${device.id} отключено")
        }

        val now = Instant.now()
        val message = DeviceCommandMessage(
            commandId = UUID.randomUUID(),
            deviceId = device.id,
            deviceType = device.typeCode,
            name = request.name,
            params = request.params,
            source = request.source,
            issuedAt = now,
            expiresAt = now.plus(CommandPublisher.COMMAND_TTL),
        )
        commandPublisher.publish(message)

        val command = Command(
            id = message.commandId,
            deviceId = device.id,
            name = message.name,
            params = message.params,
            source = message.source,
            status = "SENT",
            createdAt = now,
        )
        return ResponseEntity.accepted()
            .location(URI.create("/api/v1/devices/${device.id}/commands/${command.id}"))
            .body(command)
    }

    private fun findDevice(id: UUID): Device =
        repository.findById(id) ?: throw ResponseStatusException(HttpStatus.NOT_FOUND, "Устройство $id не найдено")
}
