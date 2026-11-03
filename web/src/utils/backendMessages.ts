import { readLanguage } from '@/stores/appearance'

// Keep unknown server details intact so diagnostics are never silently lost.
export function translateBackendMessage(message?: string): string {
  if (!message || readLanguage() !== 'en') return message ?? ''
  const replacements: Array<[string, string]> = [
    ['请先创建管理员账号', 'Create an administrator account first'],
    ['请先登录', 'Log in first'],
    ['请求方法不支持', 'HTTP method not supported'],
    ['登录失败次数过多，请稍后重试', 'Too many failed login attempts. Try again later'],
    ['用户名或密码错误', 'Incorrect username or password'],
    ['无法读取自动选优和故障切换状态', 'Unable to read automatic-selection and fallback status'],
    ['当前内核未提供代理组状态', 'The current kernel does not provide proxy group status'],
    ['实时日志流不可用', 'Live log stream is unavailable'],
    ['更新检查未配置', 'Update checking is not configured'],
    ['更新安装未配置', 'Update installation is not configured'],
    ['检查更新失败', 'Could not check for updates'],
    ['安装更新失败', 'Could not install the update'],
    ['当前更新仓库尚无正式发布版本，或仓库地址不可用', 'No stable release is published yet, or the update repository is unavailable'],
    ['更新已在进行中', 'An update is already in progress'],
    ['容器部署请通过更新镜像升级，不能在容器内替换程序', 'Update the container image; the executable cannot be updated inside the container'],
    ['当前程序目录不可写，请使用有权限的更新方式', 'The install directory is not writable. Use an update method with the required permissions'],
    ['无法解析当前程序安装路径', 'Could not resolve the current executable path'],
    ['删除运行中的节点前请先停止代理', 'Stop the active proxy before deleting these nodes'],
    ['删除节点前请先移除 SOCKS5 端口', 'Remove the SOCKS5 port binding before deleting this node'],
    ['SOCKS5 端口无效或与混合端口冲突', 'The SOCKS5 port is invalid or conflicts with a mixed port'],
    ['当前系统不支持系统代理；Linux 请确认 gsettings 与 GNOME D-Bus 会话可用，服务器环境请使用 TUN', 'System proxy is unavailable. On Linux, ensure gsettings and a GNOME D-Bus session are available; on servers, use TUN'],
    ['精简版不支持已保存的双核配置；数据保留，请重新选择 mihomo 节点', 'Lite cannot run the saved dual-kernel configuration. Data is retained; select a mihomo node again'],
    ['精简版不支持内核 ', 'Lite does not support kernel '],
    ['多节点启动需指定方式并选择 2–64 个节点', 'Multi-node startup requires a mode and 2–64 selected nodes'],
    ['负载均衡设置需要配合负载均衡启动方式', 'Load-balancing settings require load-balancing startup mode'],
    ['仅 SOCKS5 代理模式下请从节点生成配置，或先关闭该设置', 'In SOCKS5-only mode, generate configuration from nodes or disable this setting first'],
    ['节点 "', 'Node "'],
    ['" 不支持内核 ', '" is not supported by kernel '],
    ['精简版不支持该协议或内核', 'Lite does not support this protocol or kernel'],
    ['链式代理的路由只能选择 PROXY、DIRECT、REJECT 或最终出口，请调整指定节点的规则', 'Chain routing supports only PROXY, DIRECT, REJECT or the final exit. Adjust the rules for the specified node'],
    ['链式代理不能同时传入原始配置或负载均衡设置', 'A proxy chain cannot be combined with raw configuration or load-balancing settings'],
    ['无法分配本地链式桥接端口', 'Unable to allocate a local proxy-chain bridge port'],
    ['Rematch 是重匹配路由适配器，没有独立代理连接，不能作为代理跳', 'Rematch is a routing adapter without an independent proxy connection and cannot be a proxy hop'],
    ['链式代理不可用：', 'Proxy chain unavailable: '],
    ['链式代理需要 2–8 个不同节点', 'A proxy chain requires 2–8 distinct nodes'],
    ['链式代理最多支持 8 跳', 'A proxy chain supports at most 8 hops'],
    ['精简版不支持 sing-box', 'Lite does not support sing-box'],
    ['精简版不支持双内核运行模式', 'Lite does not support dual-kernel run mode'],
    ['内核生成器不可用', 'Kernel generator unavailable'],
    ['SOCKS5 端口无效或与混合端口冲突', 'The SOCKS5 port is invalid or conflicts with a mixed port'],
    ['此 Snell 版本需要 mihomo 内核', 'This Snell version requires mihomo'],
    ['此 Snell 版本需要 sing-box 内核', 'This Snell version requires sing-box'],
    ['MASQUE 需要 mihomo 内核', 'MASQUE requires mihomo'],
    ['不支持的负载均衡策略', 'Unsupported load-balancing strategy'],
    ['健康检查地址必须是有效的 HTTP 或 HTTPS URL', 'The health-check address must be a valid HTTP or HTTPS URL'],
    ['健康检查间隔需为 10–86400 秒，超时需为 100–60000 毫秒', 'Health-check interval must be 10–86400 seconds and timeout must be 100–60000 milliseconds'],
    ['请选择 2–64 个不同节点', 'Select 2–64 distinct nodes'],
    ['负载均衡需要使用 mihomo 内核', 'Load balancing requires mihomo'],
    ['缺少负载均衡设置', 'Load-balancing settings are missing'],
    ['不支持的多节点启动方式', 'Unsupported multi-node startup mode'],
    ['节点不存在', 'Node not found'],
    ['当前运行环境不支持双内核链式代理', 'The current runtime does not support dual-kernel proxy chains'],
    ['运行环境不支持双内核', 'The runtime does not support dual kernels'],
    ['精简版不支持该协议或内核', 'Lite does not support this protocol or kernel'],
    ['组网/VPN 节点只能访问服务端允许的网段；访问公网需配置出口节点或默认路由', 'Network/VPN nodes can access only subnets allowed by the server; configure an exit node or default route for internet access'],
    ['当前链路仅支持 TCP，走代理链的 UDP 请求会被拒绝；直连规则保持生效', 'This chain supports TCP only; UDP requests through it are rejected. Direct rules remain active'],
    ['ShadowTLS 是传输包装，后面必须紧接 Shadowsocks，不能作为最终出口', 'ShadowTLS is a transport wrapper and must be immediately followed by Shadowsocks; it cannot be the final exit'],
    ['ShadowTLS 与其后的 Shadowsocks 必须使用同一内核', 'ShadowTLS and the following Shadowsocks hop must use the same kernel'],
    ['Rematch 是重匹配路由适配器，没有独立代理连接，不能作为代理跳', 'Rematch is a routing adapter without an independent proxy connection and cannot be a proxy hop'],
    ['链式代理需要 2–8 个不同节点', 'A proxy chain requires 2–8 distinct nodes'],
    ['请选择 2–64 个不同节点', 'Select 2–64 distinct nodes'],
    ['请勿重复选择同一个节点', 'Do not select the same node more than once'],
    ['所选节点没有可用的内核组合', 'No kernel combination is available for the selected nodes'],
    ['负载均衡需要使用 mihomo 内核', 'Load balancing requires mihomo'],
    ['缺少负载均衡设置', 'Load balancing settings are missing'],
    ['不支持的负载均衡策略', 'Unsupported load balancing strategy'],
    ['健康检查地址必须是有效的 HTTP 或 HTTPS URL', 'The health check address must be a valid HTTP or HTTPS URL'],
    ['健康检查间隔需为 10–86400 秒，超时需为 100–60000 毫秒', 'The health check interval must be 10–86400 seconds and timeout must be 100–60000 milliseconds'],
    ['链式代理最多支持 8 跳', 'A proxy chain supports at most 8 hops'],
    ['不支持的多节点启动方式', 'Unsupported multi-node mode'],
    ['节点不存在', 'Node not found'],
    ['MASQUE 需要 mihomo 内核', 'MASQUE requires mihomo'],
    ['此 Snell 版本需要 mihomo 内核', 'This Snell version requires mihomo'],
    ['此 Snell 版本需要 sing-box 内核', 'This Snell version requires sing-box'],
    ['精简版不支持已保存的双核配置；数据保留，请重新选择 mihomo 节点', 'Lite does not support the saved dual-kernel configuration; data is retained. Select a mihomo node'],
    ['链式代理不能同时传入原始配置或负载均衡设置', 'A proxy chain cannot be combined with raw configuration or load balancing settings'],
    ['当前运行环境不支持双内核链式代理', 'This environment does not support dual-kernel proxy chains'],
    ['链式代理的路由只能选择 PROXY、DIRECT、REJECT 或最终出口，请调整指定节点的规则', 'Chain routes may use only PROXY, DIRECT, REJECT or the final exit; adjust the specified node rules'],
    ['SOCKS5 端口无效或与混合端口冲突', 'The SOCKS5 port is invalid or conflicts with a mixed port'],
    ['内核生成器不可用', 'Kernel generator unavailable'],
    ['运行环境不支持双内核', 'This environment does not support dual kernels'],
    ['无法分配本地链式桥接端口', 'Unable to allocate a local chain bridge port'],
    ['精简版不支持该协议或内核', 'Lite does not support this protocol or kernel'],
    ['链式代理不可用：', 'Proxy chain unavailable: ']
  ]
  let result = message
  for (const [zh, en] of replacements) result = result.split(zh).join(en)
  return result
    .replace(/精简版不支持 (.+)/g, 'Lite does not support $1')
    .replace(/第 (\d+) 跳未能生成配置/g, 'Hop $1 could not generate configuration')
    .replace(/第 (\d+) 跳「([^」]+)」参数无法生成内核配置：/g, 'Hop $1 "$2" has options that could not be converted to kernel configuration: ')
    .replace(/第 (\d+) 跳「([^」]+)」需要 UDP 传输，但前一跳「([^」]+)」无法转发 UDP；请更换入口或调整顺序/g, 'Hop $1 "$2" requires UDP, but previous hop "$3" cannot forward UDP; change the entry or reorder the chain')
    .replace(/第 (\d+) 跳「([^」]+)」：/g, 'Hop $1 "$2": ')
    .replace(/第 (\d+) 跳：/g, 'Hop $1: ')
    .replace(/不支持 (\S+) 内核/g, 'Unsupported kernel: $1')
    .replace(/未知协议 (\S+)/g, 'Unknown protocol: $1')
}
