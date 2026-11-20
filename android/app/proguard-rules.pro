# gomobile bindings are called from native code.
-keep class go.** { *; }
-keep class io.nuc134r.gocore.** { *; }
# The Box implementation is called from Go through the interface.
-keep class * implements io.nuc134r.gocore.mobile.Box { *; }
# kotlinx.serialization
-keepattributes *Annotation*, InnerClasses
-keepclassmembers class io.nuc134r.sockmnezia.** {
    *** Companion;
}
-keepclasseswithmembers class io.nuc134r.sockmnezia.** {
    kotlinx.serialization.KSerializer serializer(...);
}
