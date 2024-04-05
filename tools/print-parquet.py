#  Copyright (C) 2023  Intergral GmbH
#
#  This program is free software: you can redistribute it and/or modify
#  it under the terms of the GNU Affero General Public License as published by
#  the Free Software Foundation, either version 3 of the License, or
#  (at your option) any later version.
#
#  This program is distributed in the hope that it will be useful,
#  but WITHOUT ANY WARRANTY; without even the implied warranty of
#  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#  GNU Affero General Public License for more details.
#
#  You should have received a copy of the GNU Affero General Public License
#  along with this program.  If not, see <https://www.gnu.org/licenses/>.
import os.path
import sys

import pandas as pd

if __name__ == '__main__':
    """
    This is a simple utility to print the contents of a parquet file.
    
    To use pass the path to the file to open as the first argument:
    
    python print-parquet.py /path/to/parquest/file [column]
    
    NOTE: Must install pandas and pyarrow
    pip install pandas pyarrow
    """
    if len(sys.argv) < 2:
        print("Must pass file to read")
        exit(1)

    file = sys.argv[1]

    if file is None:
        print("Must pass file to read")
        exit(1)

    if not os.path.exists(file):
        print("File doesn't exist:", file)
        exit(1)

    parquet = pd.read_parquet(file)

    print("Number of Rows: ", parquet.shape[0])

    if len(sys.argv) == 3:
        sample = parquet.sample(n=5)
        print(sample[sys.argv[2]].values)
    else:
        print(parquet.sample(n=5))
