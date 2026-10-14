package ru.warmhouse.devices

import org.springframework.jdbc.core.JdbcTemplate
import org.springframework.jdbc.core.RowMapper
import org.springframework.stereotype.Repository
import java.util.UUID

@Repository
class DeviceRepository(private val jdbc: JdbcTemplate) {

    private val rowMapper = RowMapper { rs, _ ->
        Device(
            id = rs.getObject("id", UUID::class.java),
            name = rs.getString("name"),
            typeCode = rs.getString("type"),
            location = rs.getString("location"),
            status = DeviceStatus.valueOf(rs.getString("status")),
            legacySensorId = rs.getObject("legacy_sensor_id") as Int?,
            createdAt = rs.getTimestamp("created_at").toInstant(),
            updatedAt = rs.getTimestamp("updated_at").toInstant(),
        )
    }

    fun findAll(type: String?): List<Device> =
        if (type == null) {
            jdbc.query("SELECT * FROM devices ORDER BY created_at", rowMapper)
        } else {
            jdbc.query("SELECT * FROM devices WHERE type = ? ORDER BY created_at", rowMapper, type)
        }

    fun findById(id: UUID): Device? =
        jdbc.query("SELECT * FROM devices WHERE id = ?", rowMapper, id).firstOrNull()

    fun create(id: UUID, name: String, type: String, location: String?): Device {
        jdbc.update(
            "INSERT INTO devices (id, name, type, location) VALUES (?, ?, ?, ?)",
            id, name, type, location,
        )
        return findById(id)!!
    }

    fun upsertLegacy(event: LegacySensorEvent, type: String) {
        jdbc.update(
            """
            INSERT INTO devices (id, name, type, location, status, legacy_sensor_id)
            VALUES (?, ?, ?, ?, 'ACTIVE', ?)
            ON CONFLICT (id) DO UPDATE
               SET name = EXCLUDED.name, type = EXCLUDED.type, location = EXCLUDED.location,
                   status = 'ACTIVE', updated_at = NOW()
            """.trimIndent(),
            event.deviceId, event.name, type, event.location, event.legacySensorId,
        )
    }

    fun disable(id: UUID) {
        jdbc.update("UPDATE devices SET status = 'DISABLED', updated_at = NOW() WHERE id = ?", id)
    }
}
