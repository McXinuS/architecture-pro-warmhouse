# Build stage
FROM gradle:8.10.2-jdk21 AS builder

WORKDIR /app

COPY settings.gradle.kts build.gradle.kts ./
RUN gradle dependencies --no-daemon > /dev/null

COPY src ./src
RUN gradle bootJar --no-daemon

# Runtime stage
FROM eclipse-temurin:21-jre-alpine

WORKDIR /app

COPY --from=builder /app/build/libs/device-service-*.jar app.jar

EXPOSE 8082

ENTRYPOINT ["java", "-jar", "app.jar"]
