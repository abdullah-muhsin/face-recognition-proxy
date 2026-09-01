using PushSdkGateway;

var configurationPath = Environment.GetEnvironmentVariable("PUSHSDK_GATEWAY_CONFIG_PATH");
if (string.IsNullOrWhiteSpace(configurationPath) || !Path.IsPathFullyQualified(configurationPath))
{
    throw new InvalidOperationException("PUSHSDK_GATEWAY_CONFIG_PATH must name an absolute gateway configuration file.");
}

if (!File.Exists(configurationPath))
{
    throw new FileNotFoundException("The Push SDK gateway configuration file does not exist.", configurationPath);
}

var builder = WebApplication.CreateBuilder(args);
builder.Configuration.AddJsonFile(configurationPath, optional: false, reloadOnChange: false);

var gatewayOptions = builder.Configuration.GetSection(GatewayOptions.SectionName).Get<GatewayOptions>()
    ?? throw new InvalidOperationException("The gateway configuration does not contain a Gateway section.");
gatewayOptions.Validate();

builder.WebHost.ConfigureKestrel(serverOptions =>
{
    serverOptions.Limits.MaxRequestBodySize = gatewayOptions.MaxDeviceRequestBytes;
});

builder.Services.AddSingleton(gatewayOptions);
builder.Services.AddSingleton<DeviceRegistry>();
builder.Services.AddSingleton<SessionStore>();
builder.Services.AddSingleton<AttendanceEventParser>();
builder.Services.AddSingleton<GatewayDatabase>();
builder.Services.AddSingleton<PushProtocolHandler>();
builder.Services.AddHttpClient<LaravelReceiverClient>(client =>
{
    client.BaseAddress = gatewayOptions.Laravel.ParseBaseUri();
    client.Timeout = TimeSpan.FromSeconds(30);
});
builder.Services.AddHostedService<DeliveryWorker>();

var app = builder.Build();
await app.Services.GetRequiredService<GatewayDatabase>().InitializeAsync(app.Lifetime.ApplicationStopping);

app.MapGet("/healthz", () => Results.Json(new { status = "ok" }));

const string pushRoute = "/iot/{terminalSerialNumber}/global/0-global/model/service/operate/PUSH";
app.MapPost(pushRoute + "/AuthInfo", HandleAuthInfoAsync);
app.MapPost(pushRoute + "/Login", HandleLoginAsync);
app.MapPost(pushRoute + "/CommandRequest", HandleCommandRequestAsync);
app.MapPost(pushRoute + "/CommandResult", HandleCommandResultAsync);
app.MapPost(pushRoute + "/Event", HandleEventAsync);
app.MapPost(pushRoute + "/Logout", HandleLogoutAsync);

app.Run();

static async Task HandleAuthInfoAsync(HttpContext context, string terminalSerialNumber, PushProtocolHandler handler, CancellationToken cancellationToken)
{
    await WriteReplyAsync(context, () => handler.AuthenticateInfoAsync(context, terminalSerialNumber, cancellationToken), cancellationToken);
}

static async Task HandleLoginAsync(HttpContext context, string terminalSerialNumber, PushProtocolHandler handler, CancellationToken cancellationToken)
{
    await WriteReplyAsync(context, () => handler.LoginAsync(context, terminalSerialNumber, cancellationToken), cancellationToken);
}

static async Task HandleCommandRequestAsync(HttpContext context, string terminalSerialNumber, PushProtocolHandler handler, CancellationToken cancellationToken)
{
    await WriteReplyAsync(context, () => handler.CommandRequestAsync(context, terminalSerialNumber, cancellationToken), cancellationToken);
}

static async Task HandleCommandResultAsync(HttpContext context, string terminalSerialNumber, PushProtocolHandler handler, CancellationToken cancellationToken)
{
    await WriteReplyAsync(context, () => handler.CommandResultAsync(context, terminalSerialNumber, cancellationToken), cancellationToken);
}

static async Task HandleEventAsync(HttpContext context, string terminalSerialNumber, PushProtocolHandler handler, CancellationToken cancellationToken)
{
    await WriteReplyAsync(context, () => handler.EventAsync(context, terminalSerialNumber, cancellationToken), cancellationToken);
}

static async Task HandleLogoutAsync(HttpContext context, string terminalSerialNumber, PushProtocolHandler handler, CancellationToken cancellationToken)
{
    await WriteReplyAsync(context, () => handler.LogoutAsync(context, terminalSerialNumber, cancellationToken), cancellationToken);
}

static async Task WriteReplyAsync(HttpContext context, Func<Task<DeviceReply>> action, CancellationToken cancellationToken)
{
    try
    {
        await DeviceReplyWriter.WriteAsync(context, await action(), cancellationToken);
    }
    catch (ProtocolException exception)
    {
        await DeviceReplyWriter.WriteAsync(context, DeviceReply.FromJson(exception.StatusCode, System.Text.Json.JsonSerializer.Serialize(new
        {
            status = exception.StatusCode,
            code = "0x00100002",
            errorMsg = exception.Message,
        })), cancellationToken);
    }
}

public partial class Program
{
}
