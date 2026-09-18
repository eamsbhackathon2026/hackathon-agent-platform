// Chuyển manifest /agent/tools và /openapi.json của một service finance-demo
// thành payload HttpToolCreateRequest.
//
// Hai nguồn phục vụ hai mục đích: OpenAPI quyết định hình dạng (kiểu, bắt buộc,
// vị trí), manifest quyết định cách diễn đạt (mô tả tiếng Việt cho mô hình đọc).

const PRIMITIVES = new Set(['string', 'number', 'integer', 'boolean']);

// Các service khai object trần, không công bố `properties`. Bảng này là hình dạng
// do người đọc mô tả của họ rồi khai lại bằng tay, không phải thứ suy ra từ máy:
// "Cờ phiên từ app: screen_sharing, remote_app, new_device, on_call,
// accessibility_service (bool)". Khi bên cung cấp đổi, phải sửa ở đây.
// Chỉ khai những hình dạng thực sự biết; object tự do như `payload` hay `meta`
// cố ý để trống.
const KNOWN_OBJECT_FIELDS = {
  session_flags: [
    { name: 'screen_sharing', type: 'boolean', description: 'Màn hình khách đang được chia sẻ', required: false },
    { name: 'remote_app', type: 'boolean', description: 'Có ứng dụng điều khiển từ xa đang chạy', required: false },
    { name: 'new_device', type: 'boolean', description: 'Giao dịch đi từ thiết bị lạ', required: false },
    { name: 'on_call', type: 'boolean', description: 'Khách đang trong một cuộc gọi', required: false },
    { name: 'accessibility_service', type: 'boolean', description: 'Dịch vụ trợ năng lạ đang bật', required: false },
  ],
};
const MAX_DISPLAY_NAME = 200;
const MAX_DESCRIPTION = 4000;

/** Gỡ anyOf/oneOf của FastAPI cho trường tùy chọn: `anyOf: [{type: X}, {type: null}]`. */
function unwrapNullable(schema) {
  if (!schema || typeof schema !== 'object') return { schema: {}, nullable: false };
  const alternatives = schema.anyOf ?? schema.oneOf;
  if (!Array.isArray(alternatives)) return { schema, nullable: false };
  const concrete = alternatives.find((item) => item?.type && item.type !== 'null');
  return { schema: concrete ?? {}, nullable: alternatives.some((item) => item?.type === 'null') };
}

function resolveRef(schema, components) {
  if (schema?.$ref) return components[schema.$ref.split('/').pop()] ?? {};
  return schema ?? {};
}

/** Kiểu nào không biểu diễn được thì hạ về string, kèm ghi chú để in ra cuối. */
function paramType(schema, notes, label) {
  const { schema: concrete } = unwrapNullable(schema);
  const type = concrete.type;
  if (PRIMITIVES.has(type) || type === 'object' || type === 'array') return type;
  notes.push(`${label}: kiểu \`${type ?? 'không khai'}\` không nhận ra, dùng text`);
  return 'string';
}

/** Kiểu phần tử của mảng suy ra từ `items.type`; bỏ trống nghĩa là phần tử tùy ý. */
function itemType(schema) {
  const { schema: concrete } = unwrapNullable(schema);
  const items = unwrapNullable(concrete.items).schema;
  if (PRIMITIVES.has(items.type) || items.type === 'object') return items.type;
  return undefined;
}

// Gợi ý kiểu trần trong manifest (`int`, `str (tùy chọn)`) không nói thêm điều gì so
// với kiểu đã khai, nên bị bỏ; `int, mã CIF` hay `MOBILE|INTERNET` thì giữ lại.
const BARE_TYPE_HINT = /^(int|integer|str|string|number|float|bool|boolean|object|array|dict|list(\[[\w\s]*\])?)(\s*\(tùy chọn\))?$/iu;

/** Lấy mô tả người viết, kiểm cả schema ngoài lẫn nhánh đã gỡ anyOf. */
function proseFrom(...candidates) {
  for (const candidate of candidates) {
    for (const source of [candidate, unwrapNullable(candidate).schema]) {
      const value = source?.description;
      if (typeof value === 'string' && value.trim()) return value.trim();
    }
  }
  return '';
}

/** Title của FastAPI chỉ là tên đã viết hoa, chỉ dùng khi nó nói thêm điều gì đó. */
function titleFrom(name, ...candidates) {
  for (const candidate of candidates) {
    for (const source of [candidate, unwrapNullable(candidate).schema]) {
      const value = source?.title;
      if (typeof value !== 'string' || !value.trim()) continue;
      if (value.trim().toLowerCase().replace(/\s+/gu, '_') === name.toLowerCase()) continue;
      return value.trim();
    }
  }
  return '';
}

function describe(name, manifestSource, ...candidates) {
  const raw = manifestSource?.[name];
  const hint = typeof raw === 'string' && raw.trim() && !BARE_TYPE_HINT.test(raw.trim()) ? raw.trim() : '';
  const prose = proseFrom(...candidates);
  // Manifest hay khai enum ở chỗ OpenAPI bỏ trống; giữ cả hai khi chúng bổ sung nhau.
  if (prose && hint && hint.includes('|') && !prose.includes('|')) return `${prose} (${hint})`;
  if (prose) return prose;
  if (hint) return hint;
  return titleFrom(name, ...candidates);
}

// Nhãn tiến trình: cụm động từ KHÔNG chủ ngữ, không viết hoa chữ đầu. Giao diện
// ghép hai vế từ cùng một chuỗi — "Em đang xem lịch sử giao dịch…" lúc trợ lý
// đang chạy và "Em đã xem lịch sử giao dịch và tổng hợp thu chi." khi kể lại —
// nên nhãn viết thành câu đủ chủ ngữ sẽ đọc thành "Em đang Em xem…".
//
// Viết tay chứ không suy ra từ mô tả manifest, vì mô tả ở đó vừa lẫn lộn cụm danh
// từ với cụm động từ, vừa đầy khái niệm nội bộ (digital twin, baseline, persona,
// playbook, decision_id) mà khách không cần biết.
//
// Công cụ mới chưa có trong bảng này nhận chuỗi rỗng, và nền tảng sẽ rơi về tên
// hiển thị — xấu hơn nhưng không sai. Sửa nhanh một nhãn thì vào màn công cụ của
// admin, bảng này chỉ là giá trị khởi tạo.
export const STEP_LABELS = {
  get_transactions: 'xem lịch sử giao dịch của anh/chị',
  get_monthly_summary: 'tổng hợp thu chi theo tháng',
  get_cashflow_forecast: 'dự báo dòng tiền các tháng tới',
  get_baseline_metrics: 'xem thói quen chi tiêu của anh/chị',
  create_transaction: 'ghi nhận giao dịch',
  update_transaction_status: 'cập nhật trạng thái giao dịch',
  get_insights: 'xem nhận định chi tiêu của kỳ này',
  generate_insights: 'phân tích lại chi tiêu trong kỳ',
  list_products: 'xem các sản phẩm đang mở bán',
  generate_recommendations: 'tìm gợi ý tích lũy phù hợp với anh/chị',
  get_recommendations: 'xem các gợi ý đã đưa cho anh/chị',
  update_recommendation: 'ghi nhận lựa chọn của anh/chị',
  list_customers: 'tra danh sách khách hàng',
  get_customer: 'xem hồ sơ khách hàng',
  get_portfolio: 'xem toàn cảnh tài chính của anh/chị',
  get_accounts: 'xem số dư tài khoản của anh/chị',
  get_baseline: 'xem thói quen giao dịch thường ngày',
  recompute_baseline: 'cập nhật lại thói quen giao dịch',
  resolve_beneficiary: 'tra thông tin người nhận',
  list_beneficiaries: 'xem sổ người nhận của anh/chị',
  set_beneficiary_status: 'đánh dấu người nhận đáng ngờ',
  list_recent_events: 'xem các thay đổi gần đây trên tài khoản',
  create_event: 'ghi nhận thay đổi trên tài khoản',
  precheck_transfer: 'kiểm tra rủi ro của lệnh chuyển tiền',
  record_intervention: 'ghi nhận câu trả lời của anh/chị',
  take_action: 'thực hiện hành động anh/chị chọn',
  get_decision: 'xem lại kết quả kiểm tra rủi ro',
  list_decisions: 'tra lịch sử cảnh báo rủi ro',
  ops_summary: 'tổng hợp số liệu vận hành',
  ops_decision_log: 'xem nhật ký cảnh báo',
  list_scams: 'tra danh sách thủ đoạn lừa đảo',
  get_scam: 'xem chi tiết một thủ đoạn lừa đảo',
  match_scam: 'đối chiếu với các thủ đoạn lừa đảo đã biết',
  get_questions: 'chuẩn bị câu hỏi xác minh',
  record_action: 'ghi nhận hành động bảo vệ',
  create_case: 'mở yêu cầu hỗ trợ cho anh/chị',
  list_cases: 'xem các yêu cầu hỗ trợ',
  update_case: 'cập nhật yêu cầu hỗ trợ',
  submit_feedback: 'ghi nhận phản hồi',
  apply_feedback: 'cập nhật hệ thống theo phản hồi',
  send_notification: 'gửi thông báo cho anh/chị',
  log_llm_call: 'ghi nhật ký kiểm toán',
  llm_trace_stats: 'xem thống kê chất lượng trả lời',
};

/** Câu đầu của mô tả, dùng làm nhãn hiển thị; cắt theo ranh giới từ. */
function displayName(description, fallback) {
  const source = (description ?? '').trim();
  if (!source) return fallback;
  const sentence = source.split(/(?<=[.!?])\s/u)[0].trim() || source;
  if (sentence.length <= MAX_DISPLAY_NAME) return sentence;
  const cut = sentence.slice(0, MAX_DISPLAY_NAME);
  return cut.slice(0, cut.lastIndexOf(' ') > 0 ? cut.lastIndexOf(' ') : MAX_DISPLAY_NAME).trim();
}

function buildParams(tool, operation, components, notes) {
  const params = [];
  const method = tool.method.toUpperCase();

  for (const match of tool.path.matchAll(/\{([^{}]+)\}/gu)) {
    const name = match[1];
    const declared = (operation.parameters ?? []).find((item) => item.in === 'path' && item.name === name);
    params.push({
      name,
      type: paramType(declared?.schema, notes, `${tool.name}.${name}`),
      description: describe(name, tool.parameters, declared, declared?.schema),
      required: true,
      in: 'path',
    });
  }

  for (const declared of operation.parameters ?? []) {
    if (declared.in !== 'query') continue;
    params.push({
      name: declared.name,
      type: paramType(declared.schema, notes, `${tool.name}.${declared.name}`),
      description: describe(declared.name, tool.parameters, declared, declared.schema),
      required: declared.required === true,
      in: 'query',
    });
  }

  const body = resolveRef(operation.requestBody?.content?.['application/json']?.schema, components);
  const requiredBody = new Set(body.required ?? []);
  for (const [name, schema] of Object.entries(body.properties ?? {})) {
    const type = paramType(schema, notes, `${tool.name}.${name}`);
    if ((type === 'object' || type === 'array') && method === 'GET') {
      throw new Error(`${tool.name}: trường \`${name}\` kiểu ${type} không gửi được trên một yêu cầu GET`);
    }
    // Với GET, request builder đẩy mọi tham số thành query dù khai `in` là gì.
    // Khai đúng ngay từ đầu để cấu hình phản ánh thứ thực sự được gửi.
    const param = {
      name,
      type,
      description: describe(name, tool.body_schema, schema),
      required: requiredBody.has(name),
      in: method === 'GET' ? 'query' : 'body',
    };
    if (type === 'array') {
      const element = itemType(schema);
      if (element) param.item_type = element;
      else notes.push(`${tool.name}.${name}: mảng không khai kiểu phần tử, để phần tử tùy ý`);
    }
    if (type === 'object') {
      const known = KNOWN_OBJECT_FIELDS[name];
      if (known) {
        param.fields = known.map((field) => ({ ...field }));
        notes.push(`${tool.name}.${name}: dùng hình dạng khai sẵn (${known.length} trường)`);
      } else {
        // Không đoán khóa từ chuỗi mô tả tiếng Việt: cách đó gãy khi bên cung cấp
        // đổi câu chữ, và sai lặng lẽ thì khó phát hiện hơn là để trống.
        notes.push(`${tool.name}.${name}: object chưa biết hình dạng, để tự do; khai tay trên giao diện nếu cần`);
      }
    }
    params.push(param);
  }

  // Manifest và OpenAPI là hai tài liệu tách rời của cùng một service. Nếu chúng lệch
  // nhau, tham số chỉ có ở manifest sẽ biến mất mà không ai biết.
  const emitted = new Set(params.map((item) => item.name));
  const declared = [...Object.keys(tool.parameters ?? {}), ...Object.keys(tool.body_schema ?? {})];
  for (const name of declared) {
    if (!emitted.has(name)) {
      notes.push(`${tool.name}.${name}: manifest có khai nhưng OpenAPI không có, tham số này bị bỏ`);
    }
  }
  return params;
}

/**
 * Dựng danh sách payload tool cho một service.
 * @returns {{tools: object[], notes: string[]}}
 */
export function buildToolPayloads({ manifest, openapi, connectionId, timeoutSeconds = 10 }) {
  const components = openapi.components?.schemas ?? {};
  const notes = [];
  const tools = manifest.tools.map((tool) => {
    const method = tool.method.toUpperCase();
    const operation = openapi.paths?.[tool.path]?.[method.toLowerCase()];
    if (!operation) {
      throw new Error(`${tool.name}: không tìm thấy ${method} ${tool.path} trong OpenAPI của ${manifest.service}`);
    }
    const description = [tool.description?.trim(), tool.returns ? `Trả về: ${tool.returns.trim()}` : '']
      .filter(Boolean)
      .join('\n')
      .slice(0, MAX_DESCRIPTION);
    return {
      slug: tool.name,
      display_name: displayName(tool.description, tool.name),
      step_label: STEP_LABELS[tool.name] ?? '',
      description,
      method,
      url_template: tool.path,
      connection_id: connectionId,
      params: buildParams(tool, operation, components, notes),
      timeout_seconds: timeoutSeconds,
    };
  });
  return { tools, notes };
}

/** So sánh phần cấu hình do script quản lý, bỏ qua trường chỉ đọc của máy chủ. */
export function toolDiffers(existing, desired) {
  const shape = (tool) => JSON.stringify({
    display_name: tool.display_name,
    step_label: tool.step_label ?? '',
    description: tool.description,
    method: tool.method,
    url_template: tool.url_template,
    connection_id: tool.connection_id ?? null,
    timeout_seconds: tool.timeout_seconds,
    params: (tool.params ?? []).map((param) => ({
      name: param.name,
      type: param.type,
      description: param.description,
      required: param.required,
      in: param.in,
      item_type: param.item_type ?? null,
      // Máy chủ trả khóa theo bảng chữ cái; dựng lại theo thứ tự cố định để so nội dung.
      fields: (param.fields ?? []).map((field) => ({
        name: field.name,
        type: field.type,
        description: field.description,
        required: field.required,
      })),
    })),
  });
  return shape(existing) !== shape(desired);
}
