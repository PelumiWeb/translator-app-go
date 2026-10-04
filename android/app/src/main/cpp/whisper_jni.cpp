// JNI bridge between WhisperContext.kt and whisper.cpp.
//
// Every function here is static on the Kotlin side and takes the context as
// a handle (the pointer as a long). The Kotlin class owns that handle and is
// responsible for never using it after free and never from two threads.

#include <jni.h>

#include <android/log.h>

#include <string>

#include "whisper.h"

#define LOG_TAG "ptranslate-whisper"
#define LOGE(...) __android_log_print(ANDROID_LOG_ERROR, LOG_TAG, __VA_ARGS__)

namespace {

whisper_context *toContext(jlong handle) {
    return reinterpret_cast<whisper_context *>(handle);
}

// Holds a Java string as UTF-8 chars and releases it when it goes out of
// scope, including on early returns.
class Utf8Chars {
public:
    Utf8Chars(JNIEnv *env, jstring string)
        : env_(env), string_(string), chars_(env->GetStringUTFChars(string, nullptr)) {}
    ~Utf8Chars() {
        if (chars_ != nullptr) env_->ReleaseStringUTFChars(string_, chars_);
    }
    Utf8Chars(const Utf8Chars &) = delete;
    Utf8Chars &operator=(const Utf8Chars &) = delete;

    const char *get() const { return chars_; }

private:
    JNIEnv *env_;
    jstring string_;
    const char *chars_;
};

}  // namespace

extern "C" {

// Returns 0 if the model cannot be loaded.
JNIEXPORT jlong JNICALL
Java_com_example_ptranslate_core_stt_WhisperContext_nativeInit(JNIEnv *env, jclass, jstring modelPath) {
    Utf8Chars path(env, modelPath);
    if (path.get() == nullptr) return 0;

    whisper_context_params params = whisper_context_default_params();
    params.use_gpu = false;

    whisper_context *context = whisper_init_from_file_with_params(path.get(), params);
    if (context == nullptr) LOGE("could not load model at %s", path.get());
    return reinterpret_cast<jlong>(context);
}

JNIEXPORT void JNICALL
Java_com_example_ptranslate_core_stt_WhisperContext_nativeFree(JNIEnv *, jclass, jlong handle) {
    whisper_free(toContext(handle));
}

// Runs the model over the samples (16 kHz mono floats in [-1, 1]) and returns
// the text as UTF-8 bytes, or null on failure.
//
// Bytes instead of a jstring: JNI's NewStringUTF expects "modified UTF-8" and
// can abort on characters outside the basic plane, which real transcripts in
// some languages contain. Kotlin decodes standard UTF-8 without that problem.
JNIEXPORT jbyteArray JNICALL
Java_com_example_ptranslate_core_stt_WhisperContext_nativeTranscribe(
        JNIEnv *env, jclass, jlong handle, jfloatArray samples, jstring language, jint threads) {
    whisper_context *context = toContext(handle);
    Utf8Chars lang(env, language);
    if (lang.get() == nullptr) return nullptr;

    whisper_full_params params = whisper_full_default_params(WHISPER_SAMPLING_GREEDY);
    params.n_threads = threads;
    params.language = lang.get();
    params.translate = false;      // keep the spoken language; ML Kit translates
    params.no_context = true;      // each recording stands alone
    params.no_timestamps = true;   // not shown anywhere, and skipping them is faster
    params.print_realtime = false;
    params.print_progress = false;
    params.print_timestamps = false;
    params.print_special = false;

    const jsize sampleCount = env->GetArrayLength(samples);
    jfloat *data = env->GetFloatArrayElements(samples, nullptr);
    if (data == nullptr) return nullptr;

    const int status = whisper_full(context, params, data, sampleCount);

    // JNI_ABORT: the samples were only read, so nothing is copied back.
    env->ReleaseFloatArrayElements(samples, data, JNI_ABORT);

    if (status != 0) {
        LOGE("whisper_full failed with status %d", status);
        return nullptr;
    }

    std::string text;
    const int segments = whisper_full_n_segments(context);
    for (int i = 0; i < segments; ++i) {
        text += whisper_full_get_segment_text(context, i);
    }

    jbyteArray bytes = env->NewByteArray(static_cast<jsize>(text.size()));
    if (bytes == nullptr) return nullptr;
    env->SetByteArrayRegion(bytes, 0, static_cast<jsize>(text.size()),
                            reinterpret_cast<const jbyte *>(text.data()));
    return bytes;
}

// The mean probability the model gave to the tokens of the last transcript,
// from 0 to 1, or -1 if there were none. Used as the confidence score.
JNIEXPORT jfloat JNICALL
Java_com_example_ptranslate_core_stt_WhisperContext_nativeMeanTokenProbability(JNIEnv *, jclass, jlong handle) {
    whisper_context *context = toContext(handle);

    // Ids from "end of text" upwards are control tokens, not words.
    const whisper_token firstSpecial = whisper_token_eot(context);

    double sum = 0;
    int count = 0;
    const int segments = whisper_full_n_segments(context);
    for (int i = 0; i < segments; ++i) {
        const int tokens = whisper_full_n_tokens(context, i);
        for (int j = 0; j < tokens; ++j) {
            if (whisper_full_get_token_id(context, i, j) >= firstSpecial) continue;
            sum += whisper_full_get_token_p(context, i, j);
            ++count;
        }
    }
    return count == 0 ? -1.0f : static_cast<jfloat>(sum / count);
}

// Which CPU features this build uses, e.g. "NEON = 1". ASCII only, so
// NewStringUTF is safe here.
JNIEXPORT jstring JNICALL
Java_com_example_ptranslate_core_stt_WhisperContext_nativeSystemInfo(JNIEnv *env, jclass) {
    return env->NewStringUTF(whisper_print_system_info());
}

}  // extern "C"
