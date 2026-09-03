//
//    Copyright (C) 2013 - 2020 Hong Jen Yee (PCMan) <pcman.tw@gmail.com>
//
//    This library is free software; you can redistribute it and/or
//    modify it under the terms of the GNU Library General Public
//    License as published by the Free Software Foundation; either
//    version 2 of the License, or (at your option) any later version.
//
//    This library is distributed in the hope that it will be useful,
//    but WITHOUT ANY WARRANTY; without even the implied warranty of
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
//    Library General Public License for more details.
//
//    You should have received a copy of the GNU Library General Public
//    License along with this library; if not, write to the
//    Free Software Foundation, Inc., 51 Franklin St, Fifth Floor,
//    Boston, MA  02110-1301, USA.
//

#include "MessageWindow.h"
#include "TextService.h"
#include "DrawUtils.h"

namespace Ime {

MessageWindow::MessageWindow(TextService* service, EditSession* session):
    ImeWindow(service),
    ghostStyle_(false),
    ghostTextColor_(RGB(96, 116, 123)),
    ghostBackgroundColor_(RGB(252, 253, 251)),
    ghostBorderColor_(RGB(213, 222, 221)),
    ghostPaddingX_(8),
    ghostPaddingY_(2),
    ghostRadius_(7) {

    HWND parent = service->compositionWindow(session);
    create(parent, WS_POPUP|WS_CLIPCHILDREN, WS_EX_TOOLWINDOW|WS_EX_TOPMOST);
}

void MessageWindow::setGhostStyle(bool enabled) {
    ghostStyle_ = enabled;
    if (!hwnd_) {
        return;
    }
    LONG_PTR exStyle = ::GetWindowLongPtrW(hwnd_, GWL_EXSTYLE);
    if (enabled) {
        exStyle |= WS_EX_TRANSPARENT | WS_EX_NOACTIVATE | WS_EX_TOOLWINDOW;
        exStyle &= ~WS_EX_LAYERED;
		margin_ = 0;
    }
    else {
        exStyle &= ~(WS_EX_TRANSPARENT | WS_EX_NOACTIVATE);
		margin_ = isImmersive() ? 10 : 5;
    }
    ::SetWindowLongPtrW(hwnd_, GWL_EXSTYLE, exStyle);
    recalculateSize();
    ::InvalidateRect(hwnd_, NULL, TRUE);
}

void MessageWindow::setGhostAppearance(COLORREF textColor, COLORREF backgroundColor, COLORREF borderColor) {
	ghostTextColor_ = textColor;
	ghostBackgroundColor_ = backgroundColor;
	ghostBorderColor_ = borderColor;
	if (hwnd_) {
		::InvalidateRect(hwnd_, NULL, TRUE);
	}
}

MessageWindow::~MessageWindow(void) {
}

// virtual
void MessageWindow::recalculateSize() {
    SIZE size = {0};
    HDC dc = GetDC(hwnd_);
    HGDIOBJ old_font = SelectObject(dc, font_);
    GetTextExtentPointW(dc, text_.c_str(), text_.length(), &size);
    SelectObject(dc, old_font);
    ReleaseDC(hwnd_, dc);

	const int width = size.cx + (ghostStyle_ ? ghostPaddingX_ * 2 : margin_ * 2);
	const int height = size.cy + (ghostStyle_ ? ghostPaddingY_ * 2 : margin_ * 2);
    SetWindowPos(hwnd_, HWND_TOPMOST, 0, 0, width, height, SWP_NOACTIVATE|SWP_NOMOVE);
	if (ghostStyle_) {
		HRGN region = ::CreateRoundRectRgn(0, 0, width + 1, height + 1,
			ghostRadius_ * 2, ghostRadius_ * 2);
		if (region && !::SetWindowRgn(hwnd_, region, TRUE)) {
			::DeleteObject(region);
		}
	}
	else {
		::SetWindowRgn(hwnd_, NULL, TRUE);
	}
}

void MessageWindow::setText(std::wstring text) {
    // FIXMEl: use different appearance under immersive mode
    text_ = text;
    recalculateSize();
    if(IsWindowVisible(hwnd_))
        InvalidateRect(hwnd_, NULL, TRUE);
}

LRESULT MessageWindow::wndProc(UINT msg, WPARAM wp, LPARAM lp) {
    switch(msg) {
    case WM_PAINT: {
            PAINTSTRUCT ps;
            BeginPaint(hwnd_, &ps);
            onPaint(ps);
            EndPaint(hwnd_, &ps);
        }
        break;
    case WM_MOUSEACTIVATE:
        return MA_NOACTIVATE;
	case WM_ERASEBKGND:
		if (ghostStyle_) {
			return 1;
		}
		break;
    default:
        return ImeWindow::wndProc(msg, wp, lp);
    }
    return 0;
}

void MessageWindow::onPaint(PAINTSTRUCT& ps) {
    int len = text_.length();
    RECT rc, textrc = {0};
    GetClientRect(hwnd_, &rc);

    // draw a flat black border in Windows 8 app immersive mode
    // draw a 3d border in desktop mode
    HDC hDC = ps.hdc;
    HFONT oldFont = (HFONT)SelectObject(hDC, font_);

    SetBkMode(hDC, TRANSPARENT);
    if (ghostStyle_) {
		HBRUSH backgroundBrush = ::CreateSolidBrush(ghostBackgroundColor_);
		HPEN borderPen = ::CreatePen(PS_SOLID, 1, ghostBorderColor_);
		HGDIOBJ oldBrush = ::SelectObject(hDC, backgroundBrush);
		HGDIOBJ oldPen = ::SelectObject(hDC, borderPen);
		::RoundRect(hDC, rc.left, rc.top, rc.right, rc.bottom,
			ghostRadius_ * 2, ghostRadius_ * 2);
		::SelectObject(hDC, oldPen);
		::SelectObject(hDC, oldBrush);
		::DeleteObject(borderPen);
		::DeleteObject(backgroundBrush);
		SetTextColor(hDC, ghostTextColor_);
		SetBkColor(hDC, ghostBackgroundColor_);
    }
    else if(isImmersive()) {
        SetTextColor(hDC, GetSysColor(COLOR_WINDOWTEXT));
        SetBkColor(hDC, GetSysColor(COLOR_WINDOW));
        HPEN pen = ::CreatePen(PS_SOLID, 3, RGB(0, 0, 0));
        HGDIOBJ oldPen = ::SelectObject(hDC, pen);
        ::Rectangle(hDC, rc.left, rc.top, rc.right, rc.bottom);
        ::SelectObject(hDC, oldPen);
        ::DeleteObject(pen);
    }
    else {
        SetTextColor(hDC, GetSysColor(COLOR_INFOTEXT));
        SetBkColor(hDC, GetSysColor(COLOR_INFOBK));
        // draw a 3d border in desktop mode
        ::FillSolidRect(hDC, &rc, ::GetSysColor(COLOR_INFOBK));
        ::Draw3DBorder(hDC, &rc, GetSysColor(COLOR_3DFACE), 0);
    }

    SIZE size;
    GetTextExtentPoint32W(hDC, text_.c_str(), len, &size);
	if (ghostStyle_) {
		rc.top += (rc.bottom - size.cy) / 2;
		rc.left += ghostPaddingX_;
	}
	else {
		rc.top += (rc.bottom - size.cy)/2;
		rc.left += (rc.right - size.cx)/2;
	}
    ExtTextOutW(hDC, rc.left, rc.top, 0, &textrc, text_.c_str(), len, NULL);

    SelectObject(hDC, oldFont);
}

} // namespace Ime
