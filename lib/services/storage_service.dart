import 'package:shared_preferences/shared_preferences.dart';
import 'crypto_service.dart';

class StorageService {
  static const _keyAccountID = 'account_id';
  static const _keyApiURL = 'api_url';
  static const _keyPrivateKey = 'wg_private_key';
  static const _keyPublicKey = 'wg_public_key';
  static const _keySelectedServer = 'selected_server';

  late SharedPreferences _prefs;

  Future<void> init() async {
    _prefs = await SharedPreferences.getInstance();
  }

  String? get accountID => _prefs.getString(_keyAccountID);
  set accountID(String? v) => v != null ? _prefs.setString(_keyAccountID, v) : _prefs.remove(_keyAccountID);

  String get apiURL => _prefs.getString(_keyApiURL) ?? 'http://localhost:8080';
  set apiURL(String v) => _prefs.setString(_keyApiURL, v);

  String? get privateKey => _prefs.getString(_keyPrivateKey);
  set privateKey(String? v) => v != null ? _prefs.setString(_keyPrivateKey, v) : _prefs.remove(_keyPrivateKey);

  String? get publicKey => _prefs.getString(_keyPublicKey);
  set publicKey(String? v) => v != null ? _prefs.setString(_keyPublicKey, v) : _prefs.remove(_keyPublicKey);

  String? get selectedServerID => _prefs.getString(_keySelectedServer);
  set selectedServerID(String? v) => v != null ? _prefs.setString(_keySelectedServer, v) : _prefs.remove(_keySelectedServer);

  bool get isLoggedIn => accountID != null && privateKey != null;

  /// Generate a WireGuard keypair using native crypto (Android/iOS)
  /// or pure Dart X25519 (desktop).
  Future<void> generateAndStoreKeypair() async {
    final keys = await CryptoService.generateKeypair();
    privateKey = keys.privateKey;
    publicKey = keys.publicKey;
  }

  Future<void> clear() async {
    await _prefs.clear();
  }
}
