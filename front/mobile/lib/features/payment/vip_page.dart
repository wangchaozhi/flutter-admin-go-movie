import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../core/api_client.dart';

class VipPage extends StatefulWidget {
  const VipPage({super.key});

  @override
  State<VipPage> createState() => _VipPageState();
}

class _VipPageState extends State<VipPage> {
  final _api = ApiClient();
  final _providers = const ['mock', 'stripe', 'paypal'];
  var _provider = 'mock';
  var _loading = true;
  var _paying = false;
  var _message = '';
  List<_Product> _products = [];

  @override
  void initState() {
    super.initState();
    _loadProducts();
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
          _message = '套餐加载失败';
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
      final data = resp['data'] as Map<String, dynamic>? ?? {};
      final checkoutUrl = data['checkout_url'] as String? ?? '';
      if (checkoutUrl.isEmpty) {
        throw Exception('支付地址为空');
      }
      final uri = Uri.parse(checkoutUrl);
      final opened = await launchUrl(uri, mode: LaunchMode.externalApplication);
      if (!opened) {
        throw Exception('无法打开支付页');
      }
      if (mounted) setState(() => _message = '支付页已打开，完成后返回应用刷新播放。');
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

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFF0D0F14),
      appBar: AppBar(
        backgroundColor: const Color(0xFF0D0F14),
        foregroundColor: Colors.white,
        title: const Text('VIP'),
      ),
      body: _loading
          ? const Center(
              child: CircularProgressIndicator(color: Color(0xFF25D0AB)),
            )
          : ListView(
              padding: const EdgeInsets.fromLTRB(16, 10, 16, 24),
              children: [
                const _VipHero(),
                const SizedBox(height: 18),
                Row(
                  children: [
                    const Text(
                      '支付方式',
                      style: TextStyle(
                        color: Color(0xFFE5E7EB),
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                    const SizedBox(width: 14),
                    DropdownButton<String>(
                      value: _provider,
                      dropdownColor: const Color(0xFF171B24),
                      style: const TextStyle(color: Colors.white),
                      items: [
                        for (final provider in _providers)
                          DropdownMenuItem(
                            value: provider,
                            child: Text(provider.toUpperCase()),
                          ),
                      ],
                      onChanged: (value) =>
                          setState(() => _provider = value ?? 'mock'),
                    ),
                  ],
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
                        '暂无可购买套餐',
                        style: TextStyle(color: Color(0xFF9CA3AF)),
                      ),
                    ),
                  ),
                if (_message.isNotEmpty) ...[
                  const SizedBox(height: 16),
                  Text(
                    _message,
                    style: const TextStyle(color: Color(0xFFF7C948)),
                  ),
                ],
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
                  'VIP 会员',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 22,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                SizedBox(height: 4),
                Text(
                  '解锁会员专属影片与更完整的观影体验',
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
                  style: const TextStyle(color: Color(0xFF9CA3AF)),
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
            child: Text(busy ? '处理中' : '购买'),
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
