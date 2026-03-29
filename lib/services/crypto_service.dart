import 'dart:io';
import 'dart:convert';
import 'dart:math';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/services.dart';

class CryptoService {
  static const _channel = MethodChannel('vpnservice/crypto');

  /// Generate a WireGuard keypair.
  /// On Android/iOS: uses native wireguard crypto via platform channel.
  /// On desktop: shells out to `wg genkey | wg pubkey`.
  static Future<({String privateKey, String publicKey})> generateKeypair() async {
    if (!kIsWeb && (Platform.isLinux || Platform.isMacOS || Platform.isWindows)) {
      return _generateDesktop();
    }
    return _generateMobile();
  }

  static Future<({String privateKey, String publicKey})> _generateMobile() async {
    final result = await _channel.invokeMethod<Map>('generateKeypair');
    if (result == null) throw Exception('Keypair generation returned null');
    return (
      privateKey: result['privateKey'] as String,
      publicKey: result['publicKey'] as String,
    );
  }

  static Future<({String privateKey, String publicKey})> _generateDesktop() async {
    // Try wg command first
    try {
      final genResult = await Process.run('wg', ['genkey']);
      if (genResult.exitCode == 0) {
        final privateKey = (genResult.stdout as String).trim();
        // wg pubkey reads from stdin, so we need to pipe
        final proc = await Process.start('wg', ['pubkey']);
        proc.stdin.write(privateKey);
        await proc.stdin.close();
        final publicKey = (await proc.stdout.transform(utf8.decoder).join()).trim();
        final exitCode = await proc.exitCode;
        if (exitCode == 0 && publicKey.isNotEmpty) {
          return (privateKey: privateKey, publicKey: publicKey);
        }
      }
    } catch (_) {
      // wg not available, fall through to pure Dart implementation
    }

    // Pure Dart fallback using X25519
    return _generatePureDart();
  }

  /// Pure Dart X25519 keypair generation.
  /// Implements the clamping and basepoint multiplication per the WireGuard spec.
  static Future<({String privateKey, String publicKey})> _generatePureDart() async {
    final random = Random.secure();
    final privateKeyBytes = Uint8List(32);
    for (var i = 0; i < 32; i++) {
      privateKeyBytes[i] = random.nextInt(256);
    }

    // Clamp private key per RFC 7748
    privateKeyBytes[0] &= 248;
    privateKeyBytes[31] &= 127;
    privateKeyBytes[31] |= 64;

    // X25519 basepoint multiplication
    final publicKeyBytes = _x25519(privateKeyBytes, _basepoint);

    return (
      privateKey: base64.encode(privateKeyBytes),
      publicKey: base64.encode(publicKeyBytes),
    );
  }

  static final Uint8List _basepoint = Uint8List.fromList([
    9, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
  ]);

  /// X25519 scalar multiplication using Montgomery ladder.
  static Uint8List _x25519(Uint8List k, Uint8List u) {
    final p = BigInt.two.pow(255) - BigInt.from(19);

    BigInt decodeUCoord(Uint8List bytes) {
      var u = BigInt.zero;
      for (var i = 0; i < 32; i++) {
        u += BigInt.from(bytes[i]) << (8 * i);
      }
      return u & ((BigInt.one << 255) - BigInt.one);
    }

    Uint8List encodeUCoord(BigInt u) {
      u = u % p;
      if (u < BigInt.zero) u += p;
      final result = Uint8List(32);
      for (var i = 0; i < 32; i++) {
        result[i] = (u & BigInt.from(0xff)).toInt();
        u >>= 8;
      }
      return result;
    }

    BigInt modInverse(BigInt a, BigInt m) {
      return a.modPow(m - BigInt.two, m);
    }

    // Decode scalar (already clamped)
    var scalar = BigInt.zero;
    for (var i = 0; i < 32; i++) {
      scalar += BigInt.from(k[i]) << (8 * i);
    }

    final uCoord = decodeUCoord(u);

    // Montgomery ladder
    var x1 = uCoord;
    var x2 = BigInt.one;
    var z2 = BigInt.zero;
    var x3 = uCoord;
    var z3 = BigInt.one;
    var swap = BigInt.zero;

    final a24 = BigInt.from(121665);

    for (var t = 254; t >= 0; t--) {
      final kt = (scalar >> t) & BigInt.one;
      swap ^= kt;

      // Conditional swap
      var dummy = swap * (x2 - x3) % p;
      x2 = (x2 - dummy) % p;
      x3 = (x3 + dummy) % p;
      dummy = swap * (z2 - z3) % p;
      z2 = (z2 - dummy) % p;
      z3 = (z3 + dummy) % p;
      swap = kt;

      final a = (x2 + z2) % p;
      final aa = (a * a) % p;
      final b = (x2 - z2) % p;
      final bb = (b * b) % p;
      final e = (aa - bb) % p;
      final c = (x3 + z3) % p;
      final d = (x3 - z3) % p;
      final da = (d * a) % p;
      final cb = (c * b) % p;
      x3 = ((da + cb) * (da + cb)) % p;
      z3 = (x1 * ((da - cb) * (da - cb))) % p;
      x2 = (aa * bb) % p;
      z2 = (e * (aa + a24 * e)) % p;
    }

    // Final conditional swap
    var dummy = swap * (x2 - x3) % p;
    x2 = (x2 - dummy) % p;
    dummy = swap * (z2 - z3) % p;
    z2 = (z2 - dummy) % p;

    final result = (x2 * modInverse(z2, p)) % p;
    return encodeUCoord(result);
  }
}
