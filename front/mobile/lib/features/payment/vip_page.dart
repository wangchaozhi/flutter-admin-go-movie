import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;
import 'package:url_launcher/url_launcher.dart';

import '../../core/api_client.dart';

class VipPage extends StatefulWidget {
  const VipPage({super.key});

  @override
  State<VipPage> createState() => _VipPageState();
}

class _VipPageState extends State<VipPage> with WidgetsBindingObserver {
  final _api = ApiClient();
  final _providers = const ['mock', 'stripe', 'paypal', 'wechat', 'alipay'];
  static const _providerLabels = {
    'mock': '模拟支付',
    'stripe': 'Stripe',
    'paypal': 'PayPal',
    'wechat': '微信支付',
    'alipay': '支付宝',
  };
  var _provider = 'mock';
  var _loading = true;
  var _paying = false;
  var _message = '';
  String? _pendingOrderNo;
  List<_Product> _products = [];
  bool _isVip = false;
  int _daysRemaining = 0;
  String? _vipUntil;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _loadProducts();
    _loadMembership();
  }

  Future<void> _loadMembership() async {
    try {
      final resp = await _api.getAuth('/api/mobile/profile');
      if (!mounted) return;
      if (resp['code'] == 0) {
        final data = resp['data'] as Map<String, dynamic>? ?? {};
        setState(() {
          _isVip = data['is_vip'] as bool? ?? false;
          _daysRemaining = (data['days_remaining'] as num?)?.toInt() ?? 0;
          _vipUntil = data['vip_until']?.toString();
        });
      }
    } catch (_) {
      // membership banner is best-effort
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      _refreshPendingOrder();
      _loadMembership();
    }
  }

  Future<void> _loadProducts() async {
    try {
      final resp = await _api.get('/api/products');
      final list = (resp['data'] as List<dynamic>? ?? [])
          .map((item) => _Product.fromJson(item as Map<String, dynamic>))
          .where((item) => item.kind == 'vip')
          .toList();
      if (mounted) {
        setState(() {
          _products = list;
          _loading = false;
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          _message = '会员套餐加载失败，请稍后重试';
          _loading = false;
        });
      }
    }
  }

  Future<void> _buy(_Product product) async {
    if (_paying) return;
    setState(() {
      _paying = true;
      _message = '';
    });
    try {
      final resp = await _api.postAuth('/api/orders', {
        'product_code': product.code,
        'provider': _provider,
      });
      _checkApiResponse(resp);
      final data = resp['data'] as Map<String, dynamic>? ?? {};
      final checkoutUrl = data['checkout_url'] as String? ?? '';
      if (checkoutUrl.isEmpty) {
        throw Exception('支付地址为空');
      }
      final orderNo = data['order_no'] as String? ?? '';
      final uri = Uri.parse(checkoutUrl);
      if (_provider == 'mock') {
        await _completeMockCheckout(_mockCheckoutUri(orderNo, uri));
        if (!mounted) return;
        setState(() => _message = '支付成功，会员权益已更新');
        await _loadMembership();
        if (!mounted) return;
        await _showPaymentSuccessDialog(product.name);
        return;
      }
      _pendingOrderNo = orderNo.isEmpty ? null : orderNo;
      final opened = await launchUrl(uri, mode: LaunchMode.externalApplication);
      if (!opened) {
        throw Exception('无法打开支付页');
      }
      if (mounted) setState(() => _message = '支付页已打开，返回应用后将自动确认结果');
    } catch (err) {
      if (mounted) {
        setState(
          () => _message = err is Exception
              ? err.toString().replaceFirst('Exception: ', '')
              : '创建订单失败',
        );
      }
    } finally {
      if (mounted) setState(() => _paying = false);
    }
  }

  void _checkApiResponse(Map<String, dynamic> resp) {
    final code = resp['code'];
    if (code == null || code == 0) return;
    throw Exception(resp['msg'] as String? ?? '请求失败');
  }

  Uri _mockCheckoutUri(String orderNo, Uri fallbackUri) {
    if (orderNo.isEmpty) return fallbackUri;
    return Uri.parse(
      '${ApiClient.baseUrl}/api/orders/${Uri.encodeComponent(orderNo)}/mock-complete',
    );
  }

  Future<void> _completeMockCheckout(Uri uri) async {
    final response = await http.get(uri);
    if (response.statusCode >= 400) {
      throw Exception('模拟支付完成失败 (${response.statusCode})');
    }
  }

  Future<void> _refreshPendingOrder() async {
    final orderNo = _pendingOrderNo;
    if (orderNo == null || orderNo.isEmpty) return;
    try {
      final resp = await _api.getAuth('/api/orders/$orderNo');
      _checkApiResponse(resp);
      final data = resp['data'] as Map<String, dynamic>? ?? {};
      final status = data['status'] as String? ?? '';
      if (!mounted) return;
      if (status == 'paid') {
        _pendingOrderNo = null;
        setState(() => _message = '支付成功，会员权益已更新');
        await _showPaymentSuccessDialog('VIP 会员');
      } else if (status == 'failed' ||
          status == 'cancelled' ||
          status == 'refunded') {
        _pendingOrderNo = null;
        setState(() => _message = '支付未完成，可以重新选择套餐');
      }
    } catch (_) {
      // Keep the pending order so the next app resume can retry confirmation.
    }
  }

  Future<void> _showPaymentSuccessDialog(String productName) {
    return showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('支付成功'),
        content: Text('$productName 已开通，可以继续观看会员内容。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('好的'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFF0D0F14),
      appBar: AppBar(
        backgroundColor: const Color(0xFF0D0F14),
        foregroundColor: Colors.white,
        title: const Text('会员中心'),
      ),
      body: _loading
          ? const Center(
              child: CircularProgressIndicator(color: Color(0xFF25D0AB)),
            )
          : ListView(
              padding: const EdgeInsets.fromLTRB(16, 10, 16, 32),
              children: [
                _MembershipStatus(
                  isVip: _isVip,
                  daysRemaining: _daysRemaining,
                  vipUntil: _vipUntil,
                ),
                const SizedBox(height: 12),
                const _VipHero(),
                const SizedBox(height: 18),
                _PaymentMethodCard(
                  provider: _provider,
                  providers: _providers,
                  providerLabels: _providerLabels,
                  onChanged: (value) => setState(() => _provider = value),
                ),
                const SizedBox(height: 12),
                for (final product in _products)
                  _VipPlan(
                    product: product,
                    busy: _paying,
                    onBuy: () => _buy(product),
                  ),
                if (_products.isEmpty)
                  const Padding(
                    padding: EdgeInsets.all(32),
                    child: Center(
                      child: Text(
                        '暂无可购买套餐，请稍后再来',
                        style: TextStyle(color: Color(0xFF9CA3AF)),
                      ),
                    ),
                  ),
                if (_message.isNotEmpty) ...[
                  const SizedBox(height: 16),
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 10,
                    ),
                    decoration: BoxDecoration(
                      color: const Color(0x22F7C948),
                      borderRadius: BorderRadius.circular(8),
                      border: Border.all(color: const Color(0x66F7C948)),
                    ),
                    child: Text(
                      _message,
                      style: const TextStyle(color: Color(0xFFF7C948)),
                    ),
                  ),
                ],
              ],
            ),
    );
  }
}

class _PaymentMethodCard extends StatelessWidget {
  const _PaymentMethodCard({
    required this.provider,
    required this.providers,
    required this.providerLabels,
    required this.onChanged,
  });

  final String provider;
  final List<String> providers;
  final Map<String, String> providerLabels;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: const Color(0xFF171B24),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0xFF2B3140)),
      ),
      child: Row(
        children: [
          const Icon(Icons.payments_outlined, color: Color(0xFF25D0AB)),
          const SizedBox(width: 10),
          const Expanded(
            child: Text(
              '支付方式',
              style: TextStyle(
                color: Color(0xFFE5E7EB),
                fontWeight: FontWeight.w800,
              ),
            ),
          ),
          DropdownButtonHideUnderline(
            child: DropdownButton<String>(
              value: provider,
              dropdownColor: const Color(0xFF171B24),
              style: const TextStyle(
                color: Colors.white,
                fontWeight: FontWeight.w800,
              ),
              iconEnabledColor: const Color(0xFF9CA3AF),
              items: [
                for (final item in providers)
                  DropdownMenuItem(
                    value: item,
                    child: Text(providerLabels[item] ?? item.toUpperCase()),
                  ),
              ],
              onChanged: (value) {
                if (value != null) onChanged(value);
              },
            ),
          ),
        ],
      ),
    );
  }
}

class _MembershipStatus extends StatelessWidget {
  const _MembershipStatus({
    required this.isVip,
    required this.daysRemaining,
    required this.vipUntil,
  });

  final bool isVip;
  final int daysRemaining;
  final String? vipUntil;

  String _formatDate(String raw) {
    final parsed = DateTime.tryParse(raw);
    if (parsed == null) return raw;
    final local = parsed.toLocal();
    String two(int n) => n.toString().padLeft(2, '0');
    return '${local.year}-${two(local.month)}-${two(local.day)}';
  }

  @override
  Widget build(BuildContext context) {
    final expiringSoon = isVip && daysRemaining <= 7;
    final String label;
    if (!isVip) {
      label = '尚未开通会员，开通后可解锁全部会员影片';
    } else {
      final until = vipUntil != null ? _formatDate(vipUntil!) : '';
      final suffix = expiringSoon ? '，即将到期，建议及时续费' : '';
      label = '会员有效期至 $until，剩余 $daysRemaining 天$suffix';
    }
    final accent = !isVip
        ? const Color(0xFF9CA3AF)
        : (expiringSoon ? const Color(0xFFF7C948) : const Color(0xFF25D0AB));
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: const Color(0xFF171B24),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: accent.withValues(alpha: 0.5)),
      ),
      child: Row(
        children: [
          Icon(
            isVip ? Icons.verified : Icons.lock_outline,
            color: accent,
            size: 20,
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              label,
              style: const TextStyle(
                color: Colors.white,
                fontSize: 13,
                height: 1.35,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _VipHero extends StatelessWidget {
  const _VipHero();

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(18),
      decoration: BoxDecoration(
        gradient: const LinearGradient(
          colors: [Color(0xFF332B12), Color(0xFF171B24)],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0x66F7C948)),
      ),
      child: Row(
        children: [
          Container(
            width: 58,
            height: 58,
            decoration: BoxDecoration(
              gradient: const LinearGradient(
                colors: [
                  Color(0xFFFFE082),
                  Color(0xFFF7C948),
                  Color(0xFFEAB308),
                ],
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
              ),
              borderRadius: BorderRadius.circular(8),
              boxShadow: const [
                BoxShadow(
                  color: Color(0x55F7C948),
                  blurRadius: 18,
                  offset: Offset(0, 8),
                ),
              ],
            ),
            child: const Icon(
              Icons.workspace_premium_rounded,
              color: Color(0xFF101318),
              size: 34,
            ),
          ),
          const SizedBox(width: 14),
          const Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '会员观影',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 22,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                SizedBox(height: 4),
                Text(
                  '完整观看会员影片，保留进度，跨端同步权益',
                  style: TextStyle(color: Color(0xFFE5E7EB), height: 1.35),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _VipPlan extends StatelessWidget {
  const _VipPlan({
    required this.product,
    required this.busy,
    required this.onBuy,
  });

  final _Product product;
  final bool busy;
  final VoidCallback onBuy;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: const Color(0xFF171B24),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0x44F7C948)),
      ),
      child: Row(
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: const Color(0x22F7C948),
              borderRadius: BorderRadius.circular(8),
            ),
            child: const Icon(Icons.diamond_rounded, color: Color(0xFFF7C948)),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  product.name,
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 17,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  product.description,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: Color(0xFF9CA3AF),
                    height: 1.35,
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  product.priceLabel,
                  style: const TextStyle(
                    color: Color(0xFFF7C948),
                    fontSize: 18,
                    fontWeight: FontWeight.w900,
                  ),
                ),
              ],
            ),
          ),
          FilledButton(
            onPressed: busy ? null : onBuy,
            style: FilledButton.styleFrom(
              backgroundColor: const Color(0xFF25D0AB),
              foregroundColor: const Color(0xFF07110F),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
            ),
            child: Text(busy ? '处理中' : '立即购买'),
          ),
        ],
      ),
    );
  }
}

class _Product {
  const _Product({
    required this.code,
    required this.name,
    required this.description,
    required this.kind,
    required this.priceCents,
    required this.currency,
  });

  factory _Product.fromJson(Map<String, dynamic> json) {
    return _Product(
      code: json['code'] as String? ?? '',
      name: json['name'] as String? ?? '',
      description: json['description'] as String? ?? '',
      kind: json['kind'] as String? ?? '',
      priceCents: json['price_cents'] as int? ?? 0,
      currency: json['currency'] as String? ?? 'USD',
    );
  }

  final String code;
  final String name;
  final String description;
  final String kind;
  final int priceCents;
  final String currency;

  String get priceLabel => '$currency ${(priceCents / 100).toStringAsFixed(2)}';
}
