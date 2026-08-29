package com.jualin.crm_employee

import io.flutter.embedding.android.FlutterFragmentActivity

// FlutterFragmentActivity, not FlutterActivity — local_auth's Android
// implementation requires a FragmentActivity to host its BiometricPrompt
// (TD §4.1, package:local_auth_android's own setup requirement).
class MainActivity : FlutterFragmentActivity()
