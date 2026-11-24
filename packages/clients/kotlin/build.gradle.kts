// Compiles and tests the generated Kotlin client (Story 1.11). Pure JVM: the client has no Android dependency; the transport is supplied by the app.
plugins {
    kotlin("jvm") version "2.2.0"
    kotlin("plugin.serialization") version "2.2.0"
}
kotlin { jvmToolchain(17) }
dependencies {
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.9.0")
    testImplementation(kotlin("test"))
}
tasks.test { useJUnit() }
