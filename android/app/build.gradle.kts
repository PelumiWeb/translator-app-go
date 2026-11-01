import java.util.Properties

plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.kotlin.serialization)
}

android {
    namespace = "com.example.ptranslate"
    compileSdk {
        version = release(36) {
            minorApiLevel = 1
        }
    }

    // Pinned so everyone builds the native code with the same compiler.
    ndkVersion = "27.1.12297006"

    defaultConfig {
        // The app's identity in the Play Store, fixed once published (ADR 0021). The
        // code keeps its own package, `namespace` above.
        applicationId = "com.pelumi.shinar"
        minSdk = 26
        targetSdk = 36
        versionCode = 1
        versionName = "1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"

        // The dev server on this machine. `make android-install` runs
        // `adb reverse`, which makes the device's localhost:8080 reach it, on
        // the emulator and on a phone over USB alike.
        buildConfigField("String", "BACKEND_URL", "\"http://localhost:8080\"")

        // The "web" OAuth client ID of the Google Cloud project, which Sign in
        // with Google writes into its tokens for the server to check. Not a
        // secret, but it belongs to the owner's project, so it is read from
        // local.properties (googleWebClientId=...) rather than committed.
        val localProperties = Properties().apply {
            rootProject.file("local.properties").takeIf { it.isFile }?.inputStream()?.use(::load)
        }
        val googleWebClientId = localProperties.getProperty("googleWebClientId", "")
        buildConfigField("String", "GOOGLE_WEB_CLIENT_ID", "\"$googleWebClientId\"")

        ndk {
            // 64-bit ARM: every current phone, and the emulator on Apple
            // silicon. Add "x86_64" here to run on an Intel emulator.
            abiFilters += "arm64-v8a"
        }
        externalNativeBuild {
            cmake {
                // Newer devices use 16 KB memory pages and refuse to load a
                // library aligned for 4 KB. NDK 27 needs to be asked.
                arguments += "-DANDROID_SUPPORT_FLEXIBLE_PAGE_SIZES=ON"
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_11
        targetCompatibility = JavaVersion.VERSION_11
    }
    externalNativeBuild {
        cmake {
            path = file("src/main/cpp/CMakeLists.txt")
            version = "3.22.1"
        }
    }
    sourceSets {
        // The device tests transcribe whisper.cpp's own sample recording, so
        // no audio file has to be copied into this repository.
        getByName("androidTest") {
            assets.srcDir("../../third_party/whisper.cpp/samples")
        }
    }
    buildFeatures {
        compose = true
        buildConfig = true
    }
}

dependencies {
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.lifecycle.runtime.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.ui.graphics)
    implementation(libs.androidx.compose.ui.tooling.preview)
    implementation(libs.androidx.compose.material3)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.kotlinx.coroutines.android)
    implementation(libs.kotlinx.serialization.json)
    implementation(libs.mlkit.translate)
    implementation(libs.okhttp)
    implementation(libs.okhttp.sse)
    implementation(libs.androidx.credentials)
    implementation(libs.androidx.credentials.play.services)
    implementation(libs.googleid)
    implementation(libs.zxing.core)
    implementation(libs.play.services.code.scanner)
    testImplementation(libs.junit)
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.okhttp.mockwebserver)
    androidTestImplementation(libs.androidx.junit)
    androidTestImplementation(libs.androidx.espresso.core)
    androidTestImplementation(platform(libs.androidx.compose.bom))
    androidTestImplementation(libs.androidx.compose.ui.test.junit4)
    debugImplementation(libs.androidx.compose.ui.tooling)
    debugImplementation(libs.androidx.compose.ui.test.manifest)
}