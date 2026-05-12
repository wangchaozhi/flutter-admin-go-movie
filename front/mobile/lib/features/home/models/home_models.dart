import 'package:flutter/material.dart';

import '../../../models/video.dart';

class HistoryEntry {
  const HistoryEntry({required this.video, required this.progress});

  factory HistoryEntry.fromJson(Map<String, dynamic> json) => HistoryEntry(
    video: Video.fromJson(json),
    progress: json['progress'] as int? ?? 0,
  );

  final Video video;
  final int progress;
}

class FavoriteEntry {
  const FavoriteEntry({required this.video});

  factory FavoriteEntry.fromJson(Map<String, dynamic> json) =>
      FavoriteEntry(video: Video.fromJson(json));

  final Video video;
}

class MobileSetting {
  const MobileSetting({
    required this.autoPlay,
    required this.wifiOnly,
    required this.preferredQuality,
  });

  factory MobileSetting.defaults() => const MobileSetting(
    autoPlay: true,
    wifiOnly: false,
    preferredQuality: 'auto',
  );

  factory MobileSetting.fromJson(Map<String, dynamic> json) => MobileSetting(
    autoPlay: json['auto_play'] as bool? ?? true,
    wifiOnly: json['wifi_only'] as bool? ?? false,
    preferredQuality: json['preferred_quality'] as String? ?? 'auto',
  );

  final bool autoPlay;
  final bool wifiOnly;
  final String preferredQuality;

  MobileSetting copyWith({
    bool? autoPlay,
    bool? wifiOnly,
    String? preferredQuality,
  }) => MobileSetting(
    autoPlay: autoPlay ?? this.autoPlay,
    wifiOnly: wifiOnly ?? this.wifiOnly,
    preferredQuality: preferredQuality ?? this.preferredQuality,
  );

  Map<String, dynamic> toJson() => {
    'auto_play': autoPlay,
    'wifi_only': wifiOnly,
    'preferred_quality': preferredQuality,
  };
}

class MobileProfile {
  const MobileProfile({
    required this.username,
    required this.nickname,
    required this.email,
    required this.status,
    required this.isVip,
    required this.vipUntil,
  });

  factory MobileProfile.fromJson(Map<String, dynamic> json) {
    final rawVipUntil = json['vip_until'] as String?;
    return MobileProfile(
      username: json['username'] as String? ?? '',
      nickname: json['nickname'] as String? ?? '',
      email: json['email'] as String? ?? '',
      status: json['status'] as String? ?? '',
      isVip: json['is_vip'] as bool? ?? false,
      vipUntil: rawVipUntil == null || rawVipUntil.isEmpty
          ? null
          : DateTime.tryParse(rawVipUntil),
    );
  }

  final String username;
  final String nickname;
  final String email;
  final String status;
  final bool isVip;
  final DateTime? vipUntil;

  String get displayName => nickname.isNotEmpty ? nickname : username;

  String get vipUntilLabel {
    final value = vipUntil;
    if (value == null) return '';
    final local = value.toLocal();
    return '${local.year}-${local.month.toString().padLeft(2, '0')}-${local.day.toString().padLeft(2, '0')}';
  }
}

class OrderSummary {
  const OrderSummary({
    required this.orderNo,
    required this.productName,
    required this.status,
    required this.amountCents,
    required this.currency,
  });

  factory OrderSummary.fromJson(Map<String, dynamic> json) {
    final product = json['product'] as Map<String, dynamic>? ?? {};
    return OrderSummary(
      orderNo: json['order_no'] as String? ?? '',
      productName: product['name'] as String? ?? '会员订单',
      status: json['status'] as String? ?? '',
      amountCents: json['amount_cents'] as int? ?? 0,
      currency: json['currency'] as String? ?? 'USD',
    );
  }

  final String orderNo;
  final String productName;
  final String status;
  final int amountCents;
  final String currency;

  String get priceLabel =>
      '$currency ${(amountCents / 100).toStringAsFixed(2)}';

  String get statusLabel => switch (status) {
    'paid' => '已支付',
    'paying' => '支付中',
    'pending' => '待支付',
    'closed' => '已关闭',
    'failed' => '失败',
    _ => status.isEmpty ? '未知' : status,
  };

  Color get statusColor => switch (status) {
    'paid' => const Color(0xFF25D0AB),
    'paying' || 'pending' => const Color(0xFFF7C948),
    'closed' || 'failed' => const Color(0xFFF87171),
    _ => const Color(0xFF9CA3AF),
  };
}
