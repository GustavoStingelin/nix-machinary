{ pkgs, ... }:

{
  services.printing.enable = true;             # CUPS
  services.printing.startWhenNeeded = true;    # socket activation

  # Driverless IPP works for many printers
  # Add drivers only if needed
  services.printing.drivers = [
    pkgs.hplipWithPlugin     #HP
    #pkgs.gutenprint
    #pkgs.epson-escpr2       # Epson
    #pkgs.brlaser            # Brother lasers that use brlaser
  ];

  hardware.sane.enable = true;
  hardware.sane.extraBackends = [
    pkgs.hplipWithPlugin
  ];

  # Optional CUPS PDF printer
  #services.printing.cups-pdf.enable = true;
}
