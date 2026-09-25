"use client"

// Bottom sheet — the phone-width stand-in for a dialog or menu (td.md §4.3).
// Written by hand on the same @base-ui Dialog primitive as ./dialog.tsx
// rather than pulled with `shadcn add sheet`: the CLI wanted to overwrite
// our customized button.tsx and installed an unrelated npm package named
// `cn` along the way (issue #160, notes.md). Only the bottom side exists —
// the one this product uses; another side gets added when a screen needs it.

import * as React from "react"
import { Dialog as SheetPrimitive } from "@base-ui/react/dialog"
import { XIcon } from "lucide-react"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"

function Sheet({ ...props }: SheetPrimitive.Root.Props) {
  return <SheetPrimitive.Root data-slot="sheet" {...props} />
}

function SheetTrigger({ ...props }: SheetPrimitive.Trigger.Props) {
  return <SheetPrimitive.Trigger data-slot="sheet-trigger" {...props} />
}

function SheetClose({ ...props }: SheetPrimitive.Close.Props) {
  return <SheetPrimitive.Close data-slot="sheet-close" {...props} />
}

function SheetContent({
  className,
  children,
  showCloseButton = true,
  ...props
}: SheetPrimitive.Popup.Props & {
  showCloseButton?: boolean
}) {
  return (
    <SheetPrimitive.Portal>
      <SheetPrimitive.Backdrop
        data-slot="sheet-overlay"
        className="fixed inset-0 isolate z-50 bg-black/20 duration-150 data-open:animate-in data-open:fade-in-0 data-closed:animate-out data-closed:fade-out-0"
      />
      <SheetPrimitive.Popup
        data-slot="sheet-content"
        className={cn(
          // Safe-area padding: the sheet's last row must clear the iPhone
          // home indicator, or it sits under a gesture zone.
          "fixed inset-x-0 bottom-0 z-50 flex max-h-[85vh] flex-col gap-3 overflow-y-auto rounded-t-2xl bg-popover p-4 pb-[calc(1rem+env(safe-area-inset-bottom))] text-sm text-popover-foreground shadow-[0_-8px_30px_oklch(0.22_0.012_60/12%)] outline-none duration-200 data-open:animate-in data-open:slide-in-from-bottom data-closed:animate-out data-closed:slide-out-to-bottom",
          className
        )}
        {...props}
      >
        {children}
        {showCloseButton && (
          <SheetPrimitive.Close
            data-slot="sheet-close"
            render={<Button variant="ghost" className="absolute top-2.5 right-2.5" size="icon-sm" />}
          >
            <XIcon />
            <span className="sr-only">Tutup</span>
          </SheetPrimitive.Close>
        )}
      </SheetPrimitive.Popup>
    </SheetPrimitive.Portal>
  )
}

function SheetTitle({ className, ...props }: SheetPrimitive.Title.Props) {
  return (
    <SheetPrimitive.Title
      data-slot="sheet-title"
      className={cn("font-heading text-base leading-none font-semibold", className)}
      {...props}
    />
  )
}

export { Sheet, SheetClose, SheetContent, SheetTitle, SheetTrigger }
