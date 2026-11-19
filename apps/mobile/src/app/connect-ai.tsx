// Connect the user's own AI (Claude, ChatGPT, any MCP client) to their library.
// One button per assistant, like "Sign in with…" buttons: Claude installs
// straight away; the others expand the extra step they need.
import * as Clipboard from 'expo-clipboard';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Linking, Pressable, ScrollView, View } from 'react-native';

import { config } from '../lib/config';
import { CHATGPT_URL, claudeInstallUrl, mcpUrl } from '../lib/mcpInstall';
import { ClaudeMark, Glyph, OpenAIMark, OtherAiMark } from '../ui/BrandMarks';
import { Button, Chip, HudLabel, Text, colors } from '../ui/module01';

const serverUrl = mcpUrl(config.apiUrl);

// Claude's brand terracotta carries ink, like Module/01's signal does.
const claudeTint = '#d97757';

type Provider = 'chatgpt' | 'other';

export default function ConnectAiScreen() {
  const [open, setOpen] = useState<Provider | null>(null);
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);

  const toggle = (p: Provider) => {
    setCopied(false);
    setOpen((cur) => (cur === p ? null : p));
  };

  const copy = async () => {
    await Clipboard.setStringAsync(serverUrl);
    setCopied(true);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setCopied(false), 4000);
  };

  return (
    <ScrollView className="bg-canvas" contentContainerClassName="p-6 gap-6">
      <HudLabel code="I-01">Conectar IA</HudLabel>
      <Text variant="bodySm">
        Tu IA busca en lo que guardaste y te responde. Corre en tu plan de Claude o ChatGPT, así que no te gasta créditos.
      </Text>

      <View className="gap-3">
        <ProviderButton
          label="Conectar con Claude"
          mark={<ClaudeMark color={colors.ink} />}
          glyph="external"
          box=""
          tint={colors.ink}
          style={{ backgroundColor: claudeTint }}
          onPress={() => Linking.openURL(claudeInstallUrl(serverUrl))}
        />

        <ProviderButton
          label="Conectar con ChatGPT"
          mark={<OpenAIMark color={colors.shell} />}
          glyph={open === 'chatgpt' ? 'collapse' : 'expand'}
          expanded={open === 'chatgpt'}
          box="bg-ink"
          tint={colors.shell}
          onPress={() => toggle('chatgpt')}
        />
        {open === 'chatgpt' ? (
          <Steps copied={copied}>
            <Text variant="bodySm">ChatGPT todavía no permite instalarlo directamente. Copiamos el link y abrimos ChatGPT para que lo pegues:</Text>
            <Text variant="bodySm">1. Configuración → Apps (o Plugins).</Text>
            <Text variant="bodySm">2. Tocá &quot;Agregar servidor MCP&quot; y pegá el link.</Text>
            <Text variant="bodySm">3. Entrá con el mismo email que usás acá.</Text>
            <Button
              onPress={async () => {
                await copy();
                await Linking.openURL(CHATGPT_URL);
              }}
            >
              Copiar link y abrir ChatGPT
            </Button>
            <Text variant="hudSm">Necesita un plan de ChatGPT que permita conectores propios.</Text>
          </Steps>
        ) : null}

        <ProviderButton
          label="Otras IAs"
          mark={<OtherAiMark color={colors.ink} />}
          glyph={open === 'other' ? 'collapse' : 'expand'}
          expanded={open === 'other'}
          box="bg-shell border border-seam"
          tint={colors.ink}
          onPress={() => toggle('other')}
        />
        {open === 'other' ? (
          <Steps copied={copied}>
            <Text variant="bodySm">Agregalo como servidor MCP remoto con este link:</Text>
            <View className="rounded-md bg-shell-recess px-4 py-3">
              <Text variant="hud" selectable>
                {serverUrl}
              </Text>
            </View>
            <Button onPress={copy}>Copiar link</Button>
          </Steps>
        ) : null}
      </View>

      <Text variant="bodySm">
        Claude y ChatGPT se conectan desde su web, porque sus apps todavía no dejan agregar conectores. Una vez conectado, también lo usás desde la app.
      </Text>
    </ScrollView>
  );
}

function ProviderButton({
  label,
  mark,
  glyph,
  expanded,
  box,
  tint,
  style,
  onPress,
}: {
  label: string;
  mark: ReactNode;
  glyph: 'external' | 'expand' | 'collapse';
  expanded?: boolean;
  box: string;
  tint: string;
  style?: { backgroundColor: string };
  onPress: () => void;
}) {
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={label}
      aria-expanded={expanded}
      onPress={onPress}
      className={`h-14 flex-row items-center gap-4 rounded-full px-6 active:translate-y-px active:opacity-90 ${box}`}
      style={style}
    >
      {mark}
      <Text variant="label" className="flex-1" style={{ color: tint }}>
        {label}
      </Text>
      <Glyph kind={glyph} color={tint} />
    </Pressable>
  );
}

function Steps({ copied, children }: { copied: boolean; children: ReactNode }) {
  return (
    <View className="gap-3 rounded-lg bg-shell p-5">
      {children}
      {copied ? <Chip tone="signal">Link copiado</Chip> : null}
    </View>
  );
}
