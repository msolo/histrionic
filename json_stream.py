from json import JSONDecoder, JSONDecodeError


class JSONStreamDecoder(object):
  dec = JSONDecoder()

  def __init__(self, data):
    self.data = data
    self.pos = 0

  def __iter__(self):
    return self

  def __next__(self):
    if self.pos >= len(self.data):
      raise StopIteration

    while self.pos < len(self.data) and self.data[self.pos].isspace():
      self.pos += 1

    if self.pos >= len(self.data):
      raise StopIteration

    try:
      obj, pos = self.dec.raw_decode(self.data, self.pos)
      self.pos = pos
    except JSONDecodeError:
      # do something sensible if there's some error
      raise
    return obj

def test():
  for x in JSONStreamDecoder('{}'):
    print(x)
  for x in JSONStreamDecoder('{}{}'):
    print(x)
  for x in JSONStreamDecoder('{}\n{}'):
    print(x)
  for x in JSONStreamDecoder('nullnull'):
    print(x)
  for x in JSONStreamDecoder('null\nnull'):
    print(x)

if __name__ == '__main__':
  test()
