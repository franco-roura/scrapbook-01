// A small native take on Module/01 (@franco_roura/module-01). The web kit is
// built on Radix and DOM elements, so these follow its guidelines instead:
// shell parts on a canvas, one notched corner, signal-orange trim, HUD
// microprint in mono, stadium buttons and sunken input wells.
import { forwardRef, type ComponentRef, type ReactNode } from 'react';
import {
  ActivityIndicator,
  Pressable,
  Text as RNText,
  TextInput,
  View,
  type PressableProps,
  type TextInputProps,
  type TextProps,
  type ViewProps,
} from 'react-native';

const cx = (...c: (string | false | null | undefined)[]) => c.filter(Boolean).join(' ');

// Hex values for the few props that can't take a className.
export const colors = {
  canvas: '#f3f4f6',
  shell: '#ffffff',
  inkSoft: '#53575e',
  signal: '#ff5f1f',
  signalInk: '#b93d0a',
  ink: '#1c1e21',
};

type Variant = 'display' | 'title' | 'body' | 'bodySm' | 'label' | 'hud' | 'hudSm';

const textVariants: Record<Variant, string> = {
  display: 'font-display text-display-md text-ink',
  title: 'font-display text-display-sm text-ink',
  body: 'font-sans text-body-md text-ink',
  bodySm: 'font-sans text-body-sm text-ink-soft',
  label: 'font-sans-bold text-label text-ink',
  hud: 'font-mono text-hud uppercase tracking-[1.4px] text-ink-soft',
  hudSm: 'font-mono text-hud-sm uppercase tracking-[1.6px] text-hud-mark',
};

export type TextStyleProps = TextProps & {
  variant?: Variant;
  className?: string;
  /** System fonts only: the iOS share extension can't load custom fonts. */
  plain?: boolean;
};

/** Text with a Module/01 type style. `plain` drops the custom font family. */
export function Text({ variant = 'body', className, plain, ...props }: TextStyleProps) {
  const base = textVariants[variant];
  return (
    <RNText
      className={cx(plain ? base.replace(/\bfont-[a-z-]+\b/, '') : base, className)}
      {...props}
      // RN's default font scaling breaks text in the iOS share extension.
      allowFontScaling={plain ? false : props.allowFontScaling}
    />
  );
}

/** The molded white part, with the top-right corner notched. */
export function Panel({
  children,
  serial,
  className,
  notch = 'md',
  plain,
  ...props
}: ViewProps & {
  children: ReactNode;
  serial?: string;
  className?: string;
  notch?: 'sm' | 'md' | false;
  plain?: boolean;
}) {
  const size = notch === 'sm' ? 10 : 20;
  return (
    <View
      className={cx('bg-shell rounded-lg p-6', notch && 'rounded-tr-none', className)}
      style={{ shadowColor: colors.ink, shadowOpacity: 0.08, shadowRadius: 16, shadowOffset: { width: 0, height: 8 }, elevation: 3 }}
      {...props}
    >
      {notch ? (
        // A canvas-colored triangle over the square corner reads as a chamfer.
        <View
          pointerEvents="none"
          style={{
            position: 'absolute',
            top: 0,
            right: 0,
            width: 0,
            height: 0,
            borderTopWidth: size,
            borderLeftWidth: size,
            borderTopColor: colors.canvas,
            borderLeftColor: 'transparent',
          }}
        />
      ) : null}
      {serial ? (
        <Text variant="hudSm" plain={plain} className="mb-3">
          {serial}
        </Text>
      ) : null}
      {children}
    </View>
  );
}

/** Mono overline: a signal code chip, the label, and a seam rule. */
export function HudLabel({ code, children, plain }: { code?: string; children: ReactNode; plain?: boolean }) {
  return (
    <View className="flex-row items-center gap-2">
      {code ? (
        <View className="bg-signal rounded-xs px-1.5 py-0.5">
          <Text variant="hudSm" plain={plain} className="text-ink">
            {code}
          </Text>
        </View>
      ) : null}
      <Text variant="hud" plain={plain}>
        {children}
      </Text>
      <View className="h-px flex-1 bg-seam" />
    </View>
  );
}

/** A small stadium tag, e.g. a platform name or category. */
export function Chip({ children, tone = 'shell', plain }: { children: ReactNode; tone?: 'shell' | 'signal'; plain?: boolean }) {
  return (
    <View className={cx('self-start rounded-full px-2.5 py-1', tone === 'signal' ? 'bg-signal-tint' : 'bg-shell-recess')}>
      <Text variant="hudSm" plain={plain} className={tone === 'signal' ? 'text-signal-ink' : 'text-ink-soft'}>
        {children}
      </Text>
    </View>
  );
}

type ButtonProps = Omit<PressableProps, 'children'> & {
  children: string;
  variant?: 'default' | 'secondary' | 'ghost' | 'destructive';
  loading?: boolean;
  plain?: boolean;
  className?: string;
};

const buttonVariants = {
  default: { box: 'bg-signal', text: 'text-ink' },
  secondary: { box: 'bg-shell border border-seam', text: 'text-ink' },
  ghost: { box: 'bg-transparent', text: 'text-signal-ink' },
  destructive: { box: 'bg-danger', text: 'text-shell' },
};

/** Stadium button. The label is ink on signal, never white. */
export function Button({ children, variant = 'default', loading, disabled, plain, className, ...props }: ButtonProps) {
  const v = buttonVariants[variant];
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ disabled: disabled || loading, busy: loading }}
      disabled={disabled || loading}
      className={cx('h-control flex-row items-center justify-center rounded-full px-6 active:translate-y-px active:opacity-90', v.box, (disabled || loading) && 'opacity-50', className)}
      {...props}
    >
      {loading ? (
        <ActivityIndicator color={colors.ink} />
      ) : (
        <Text variant="label" plain={plain} className={v.text}>
          {children}
        </Text>
      )}
    </Pressable>
  );
}

/** A label above a sunken input well, with an optional error line. */
export const Field = forwardRef<
  ComponentRef<typeof TextInput>,
  TextInputProps & { label: string; error?: string | null; plain?: boolean; className?: string }
>(function Field({ label, error, plain, className, multiline, ...props }, ref) {
  return (
    <View className="gap-2">
      <Text variant="label" plain={plain} className={error ? 'text-danger' : undefined}>
        {label}
      </Text>
      <TextInput
        ref={ref}
        multiline={multiline}
        placeholderTextColor={colors.inkSoft}
        selectionColor={colors.signalInk}
        className={cx(
          'rounded-md px-4 text-body-md text-ink',
          !plain && 'font-sans',
          error ? 'bg-danger-tint' : 'bg-shell-recess',
          multiline ? 'min-h-24 py-3' : 'h-control',
          className,
        )}
        style={multiline ? { textAlignVertical: 'top' } : undefined}
        {...props}
        allowFontScaling={plain ? false : props.allowFontScaling}
      />
      {error ? (
        <Text variant="bodySm" plain={plain} className="text-danger">
          {error}
        </Text>
      ) : null}
    </View>
  );
});
