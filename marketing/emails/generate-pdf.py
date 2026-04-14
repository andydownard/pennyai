#!/usr/bin/env python3
"""Generate a compact PDF review document of all Groflo email campaign variants."""

from fpdf import FPDF
import re
from html.parser import HTMLParser


def sanitize(text):
    replacements = {
        '\u2014': '--', '\u2013': '-', '\u2018': "'", '\u2019': "'",
        '\u201c': '"', '\u201d': '"', '\u2026': '...', '\u2022': '*',
        '\u00a0': ' ', '\u2192': '->', '\u2713': '[ok]', '\u2715': '[x]',
        '\u00b7': '*', '\u00a9': '(c)', '\U0001f610': '', '\u2028': '\n',
    }
    for k, v in replacements.items():
        text = text.replace(k, v)
    return text.encode('latin-1', 'replace').decode('latin-1')


class HTMLTextExtractor(HTMLParser):
    def __init__(self):
        super().__init__()
        self.result = []
        self.skip = False

    def handle_starttag(self, tag, attrs):
        if tag == 'style':
            self.skip = True
        elif tag in ('br',):
            self.result.append('\n')
        elif tag in ('p', 'div', 'h1', 'li', 'tr'):
            self.result.append('\n')
        elif tag in ('td', 'th'):
            self.result.append('  |  ')

    def handle_endtag(self, tag):
        if tag == 'style':
            self.skip = False
        elif tag in ('p', 'div', 'h1', 'ul', 'ol', 'table'):
            self.result.append('\n')

    def handle_data(self, data):
        if not self.skip:
            self.result.append(data)

    def get_text(self):
        text = ''.join(self.result)
        text = re.sub(r'[ \t]+', ' ', text)
        text = re.sub(r'\n{3,}', '\n\n', text)
        return text.strip()


def extract_html(filepath):
    with open(filepath, 'r') as f:
        html = f.read()
    p = HTMLTextExtractor()
    p.feed(html)
    return p.get_text()


def main():
    pdf = FPDF()
    pdf.set_auto_page_break(auto=True, margin=12)
    LM = 10  # left margin
    RM = 10
    PW = 210 - LM - RM  # printable width

    # --- COVER ---
    pdf.add_page()
    pdf.ln(30)
    pdf.set_font('Helvetica', 'B', 32)
    pdf.set_text_color(53, 100, 79)
    pdf.cell(0, 12, 'Groflo', new_x="LMARGIN", new_y="NEXT", align='C')
    pdf.ln(3)
    pdf.set_font('Helvetica', '', 18)
    pdf.set_text_color(45, 45, 45)
    pdf.cell(0, 10, 'Email Campaign Variants', new_x="LMARGIN", new_y="NEXT", align='C')
    pdf.set_font('Helvetica', '', 12)
    pdf.set_text_color(120, 120, 120)
    pdf.cell(0, 8, 'Internal Review Document - April 2026', new_x="LMARGIN", new_y="NEXT", align='C')
    pdf.ln(10)
    pdf.set_font('Helvetica', '', 10)
    pdf.cell(0, 6, '10 variants: 5 B2C (consumers) + 5 B2B (brand marketers)', new_x="LMARGIN", new_y="NEXT", align='C')
    pdf.cell(0, 6, 'Prepared for team review and A/B test planning', new_x="LMARGIN", new_y="NEXT", align='C')

    # --- VARIANT DATA ---
    b2c = [
        ("B2C-A", "No more receipt scanning. Ever.",
         "Old way (receipt uploads) vs new way (deals on your list)",
         "emails/b2c-variant-a-no-more-receipts.html"),
        ("B2C-B", "Your shopping list now saves you money.",
         "Deals hunt you -- passive savings. Flip table old vs new",
         "emails/b2c-variant-b-deals-find-you.html"),
        ("B2C-C", "Brands still want to give you free stuff.",
         "Penny Swaps -- same free products, zero friction",
         "emails/b2c-variant-c-free-groceries.html"),
        ("B2C-D", "No more 'I'm going to the store. Need anything?'",
         "Family coordination -- shared lists, no duplicates",
         "emails/b2c-variant-d-family-list.html"),
        ("B2C-E", "Stop scanning receipts for pocket change.",
         "Anti-receipt / anti-cashback. Ibotta contrast",
         "emails/b2c-variant-e-stop-scanning-receipts.html"),
    ]

    b2b = [
        ("B2B-A", "Your products. Their shopping list. Zero friction.",
         "Old funnel (6 steps) vs new (3 steps)",
         "emails/b2b-variant-a-reach-families-directly.html"),
        ("B2B-B", "Receipt marketing is dead. Here's what replaced it.",
         "Problem list vs solution list. Blunt and direct",
         "emails/b2b-variant-b-ditch-receipt-marketing.html"),
        ("B2B-C", "Shelf placement costs millions. List placement doesn't.",
         "Slotting fees vs list placement comparison",
         "emails/b2b-variant-c-shelf-to-list.html"),
        ("B2B-D", "Pay when they want it. Not when they ignore it.",
         "Performance/intent-based. Anti-wasted-impressions",
         "emails/b2b-variant-d-pay-per-intent.html"),
        ("B2B-E", "Cashback apps reach shoppers after. We reach them before.",
         "Head-to-head Ibotta comparison table",
         "emails/b2b-variant-e-ibotta-alternative.html"),
    ]

    # --- TOC (compact) ---
    pdf.ln(10)
    pdf.set_draw_color(53, 100, 79)
    pdf.set_line_width(0.5)
    pdf.line(LM, pdf.get_y(), 210 - RM, pdf.get_y())
    pdf.ln(4)

    pdf.set_font('Helvetica', 'B', 11)
    pdf.set_text_color(53, 100, 79)
    pdf.cell(0, 6, 'B2C - Former Coupon Redeemers (Consumers)', new_x="LMARGIN", new_y="NEXT")
    pdf.set_font('Helvetica', '', 8)
    pdf.set_text_color(80, 80, 80)
    for label, subject, angle, _ in b2c:
        pdf.set_x(LM + 4)
        pdf.cell(0, 4.5, sanitize(f'{label}: "{subject}" -- {angle}'), new_x="LMARGIN", new_y="NEXT")
    pdf.ln(3)

    pdf.set_font('Helvetica', 'B', 11)
    pdf.set_text_color(53, 100, 79)
    pdf.cell(0, 6, 'B2B - Former Brand Marketers (CPG)', new_x="LMARGIN", new_y="NEXT")
    pdf.set_font('Helvetica', '', 8)
    pdf.set_text_color(80, 80, 80)
    for label, subject, angle, _ in b2b:
        pdf.set_x(LM + 4)
        pdf.cell(0, 4.5, sanitize(f'{label}: "{subject}" -- {angle}'), new_x="LMARGIN", new_y="NEXT")

    # --- RENDER VARIANTS ---
    def render_section(title, variants):
        pdf.add_page()
        pdf.set_font('Helvetica', 'B', 16)
        pdf.set_text_color(53, 100, 79)
        pdf.cell(0, 8, sanitize(title), new_x="LMARGIN", new_y="NEXT", align='C')
        pdf.set_draw_color(53, 100, 79)
        pdf.set_line_width(0.5)
        mid = 105
        pdf.line(mid - 25, pdf.get_y() + 1, mid + 25, pdf.get_y() + 1)
        pdf.ln(5)

        for i, (label, subject, angle, filepath) in enumerate(variants):
            # Check if we need a new page (at least 60mm needed for a variant header + some body)
            if pdf.get_y() > 220:
                pdf.add_page()

            # Variant header bar
            pdf.set_fill_color(53, 100, 79)
            pdf.set_font('Helvetica', 'B', 8)
            pdf.set_text_color(255, 255, 255)
            pdf.cell(PW, 5, sanitize(f'  {label}  |  SUBJECT: {subject}'), fill=True, new_x="LMARGIN", new_y="NEXT")

            # Angle
            pdf.set_font('Helvetica', 'I', 7)
            pdf.set_text_color(120, 120, 120)
            pdf.cell(0, 4, sanitize(f'Angle: {angle}'), new_x="LMARGIN", new_y="NEXT")
            pdf.ln(1)

            # Body content
            pdf.set_font('Helvetica', '', 7.5)
            pdf.set_text_color(50, 50, 50)
            content = extract_html(filepath)
            lines = [l.strip() for l in content.split('\n')]
            blank_count = 0
            for line in lines:
                if not line:
                    blank_count += 1
                    if blank_count <= 1:
                        pdf.ln(1.5)
                    continue
                blank_count = 0
                pdf.set_x(LM)
                pdf.multi_cell(PW, 3.5, sanitize(line))

            # Separator between variants
            pdf.ln(3)
            if i < len(variants) - 1:
                pdf.set_draw_color(220, 220, 220)
                pdf.set_line_width(0.2)
                pdf.line(LM, pdf.get_y(), 210 - RM, pdf.get_y())
                pdf.ln(3)

    render_section('B2C Variants - Former Coupon Redeemers', b2c)
    render_section('B2B Variants - Former Brand Marketers', b2b)

    # --- A/B TESTING STRATEGY (compact) ---
    if pdf.get_y() > 200:
        pdf.add_page()
    else:
        pdf.ln(5)
        pdf.set_draw_color(53, 100, 79)
        pdf.set_line_width(0.5)
        pdf.line(LM, pdf.get_y(), 210 - RM, pdf.get_y())
        pdf.ln(4)

    pdf.set_font('Helvetica', 'B', 14)
    pdf.set_text_color(53, 100, 79)
    pdf.cell(0, 7, 'A/B Testing Strategy', new_x="LMARGIN", new_y="NEXT")
    pdf.ln(2)

    sections = [
        ("Wave 1 -- Initial Split Test (Day 1)", [
            "Send all 5 variants to equal segments (20% each) of each audience list.",
            "Measure: open rate (subject line) + click-through rate (CTA/content).",
            "Wait 48 hours for statistically meaningful results.",
        ]),
        ("Wave 2 -- Winner + Runner-Up (Day 3)", [
            "Take the top 2 performing variants from Wave 1.",
            "Re-send to non-openers with tweaked subject lines.",
            "Example: if 'No more receipts' won, follow up with 'Still photographing receipts for $0.35?'",
            "Captures people who missed or ignored the first email.",
        ]),
        ("Wave 3 -- Final Nudge (Day 10)", [
            "Short follow-up to remaining non-converters.",
            "Use a DIFFERENT angle from the Wave 1 winner.",
            "If receipts angle won, try family coordination or free products.",
            "Keep brief: 3-4 sentences, single CTA.",
        ]),
        ("Key Metrics", [
            "Open Rate -- which subject lines resonate?",
            "Click-Through Rate -- which content drives action?",
            "Conversion Rate -- who signs up at app.groflo.io?",
            "Unsubscribe Rate -- which messaging turns people off?",
            "B2B: track replies and demo requests separately.",
        ]),
        ("Audience Segmentation", [
            "B2C: consumers who uploaded receipts on old Groflo coupon platform.",
            "B2B: brand marketers who had Groflo dashboard accounts.",
            "Keep these audiences COMPLETELY separate -- different pain points, CTAs.",
            "B2C links to app.groflo.io  |  B2B links to groflo.io/for-brands.",
        ]),
    ]

    for title, bullets in sections:
        pdf.set_font('Helvetica', 'B', 9)
        pdf.set_text_color(45, 45, 45)
        pdf.cell(0, 5, sanitize(title), new_x="LMARGIN", new_y="NEXT")
        pdf.set_font('Helvetica', '', 8)
        pdf.set_text_color(80, 80, 80)
        for b in bullets:
            pdf.set_x(LM + 4)
            pdf.multi_cell(PW - 4, 3.8, sanitize(f'* {b}'))
        pdf.ln(2.5)

    out = 'emails/Groflo-Email-Campaign-Variants.pdf'
    pdf.output(out)
    print(f'PDF generated: {out}')


if __name__ == '__main__':
    main()
